#!/usr/bin/env bash
#
# Point .env at one environment, with the secrets that environment needs.
#
# The secrets live in Google Secret Manager and are read with gcloud, under the
# caller's own credentials: nothing is stored in this repository, and a person
# who cannot read a secret cannot activate the profile that needs it.
#
# Only a delimited block of .env is written. Everything outside it — a port, a
# preset, a token typed by hand — is preserved exactly, because the block is
# spliced in rather than the file rewritten.
#
# Usage:
#   scripts/set-env.sh dev                 activate the dev profile
#   scripts/set-env.sh prod                activate prod (asks for confirmation)
#   scripts/set-env.sh prod --yes          ... without asking
#   scripts/set-env.sh dev --dry-run       show what would be written, fetch nothing
#   scripts/set-env.sh dev --export        print export lines for eval, write nothing
#   scripts/set-env.sh --status            say which profile .env currently carries
#
# --export prints secret values on stdout, which is the point of it:
#   eval "$(scripts/set-env.sh dev --export)"
# Everything else keeps them off the terminal.

set -euo pipefail

BEGIN_MARK='# >>> sectile set-env >>>'
END_MARK='# <<< sectile set-env <<<'

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)

# The generated .env belongs to the checkout the script was called from, which
# in a worktree is that worktree: a server started there reads its own file.
if root=$(git -C "$script_dir" rev-parse --show-toplevel 2>/dev/null); then
    :
else
    root=$(dirname -- "$script_dir")
fi

conf_file=${SECTILE_ENV_PROFILES:-$script_dir/env-profiles.conf}
env_file=$root/.env

die() {
    printf 'set-env: %s\n' "$1" >&2
    exit 1
}

note() {
    printf '  %s\n' "$1" >&2
}

usage() {
    cat >&2 <<'USAGE'
Point .env at one environment, with the secrets that environment needs.

  scripts/set-env.sh dev              activate the dev profile
  scripts/set-env.sh prod             activate prod (asks for confirmation)
  scripts/set-env.sh prod --yes       ... without asking
  scripts/set-env.sh dev --dry-run    show what would be written, fetch nothing
  scripts/set-env.sh dev --export     print export lines for eval, write nothing
  scripts/set-env.sh --status         say which profile .env currently carries

Only the block between the set-env markers is rewritten; the rest of .env is
preserved. Secrets are read from Google Secret Manager with your own gcloud
credentials, and only --export puts them on stdout:

  eval "$(scripts/set-env.sh dev --export)"
USAGE
}

# ---------------------------------------------------------------------------
# Manifest
# ---------------------------------------------------------------------------

# profile_project echoes the Google Cloud project a profile reads from.
profile_project() {
    local profile=$1 kind rest_profile value
    while read -r kind rest_profile value _; do
        case $kind in
        project) [ "$rest_profile" = "$profile" ] && printf '%s\n' "$value" ;;
        esac
    done < <(strip_comments) | tail -n 1
}

# profile_vars echoes "VARIABLE<tab>source" for a profile, later declarations
# replacing earlier ones.
profile_vars() {
    local profile=$1 kind rest_profile name source
    while read -r kind rest_profile name source _; do
        case $kind in
        var) [ "$rest_profile" = "$profile" ] && printf '%s\t%s\n' "$name" "$source" ;;
        esac
    done < <(strip_comments) | awk -F'\t' '{ last[$1] = $2; order[$1] = order[$1] ? order[$1] : ++n }
        END { for (k in last) printf "%d\t%s\t%s\n", order[k], k, last[k] }' |
        sort -n | cut -f2-
}

strip_comments() {
    grep -v -e '^[[:space:]]*#' -e '^[[:space:]]*$' -- "$conf_file" || true
}

known_profiles() {
    strip_comments | awk '$1 == "project" { print $2 }' | sort -u
}

# ---------------------------------------------------------------------------
# Secrets
# ---------------------------------------------------------------------------

require_gcloud() {
    command -v gcloud >/dev/null 2>&1 ||
        die "gcloud est introuvable. Installez le Google Cloud CLI, puis « gcloud auth login »."
    gcloud auth list --filter=status:ACTIVE --format='value(account)' 2>/dev/null | grep -q . ||
        die "aucun compte gcloud actif. Lancez « gcloud auth login »."
}

# urlencode percent-encodes everything that is not unreserved, so a password
# carrying an @ or a / cannot cut a connection string in two. Bytes, not
# characters: a DSN is compared byte by byte by whoever parses it.
urlencode() {
    local string=$1 i c out=''
    local LC_ALL=C
    for ((i = 0; i < ${#string}; i++)); do
        c=${string:i:1}
        case $c in
        [a-zA-Z0-9.~_-]) out=$out$c ;;
        *) out=$out$(printf '%%%02X' "'$c") ;;
        esac
    done
    printf '%s' "$out"
}

# resolved_value echoes what a variable already resolved to in this run, empty
# when the profile has not declared it yet.
resolved_value() {
    local wanted=$1 i
    for ((i = 0; i < ${#names[@]}; i++)); do
        if [ "${names[i]}" = "$wanted" ]; then
            printf '%s' "${values[i]}"
            return 0
        fi
    done
    return 1
}

# compose expands ${VAR} against the variables already resolved, percent-encoding
# each substitution. The literal part of the template is left alone: that is what
# carries the scheme, the separators and the query string.
compose() {
    local template=$1 out='' rest=$1 name substituted
    out=''
    rest=$template
    while [ -n "$rest" ]; do
        case $rest in
        *'${'*)
            out=$out${rest%%'${'*}
            rest=${rest#*'${'}
            case $rest in
            *'}'*) : ;;
            *) die "modèle compose mal formé, « } » manquant : $template" ;;
            esac
            name=${rest%%'}'*}
            rest=${rest#*'}'}
            if ! substituted=$(resolved_value "$name"); then
                die "compose : $name n'est pas déclaré avant son usage dans « $template »."
            fi
            out=$out$(urlencode "$substituted")
            ;;
        *)
            out=$out$rest
            rest=''
            ;;
        esac
    done
    printf '%s' "$out"
}

# fetch_secret echoes one secret's value. The name may carry a #version.
fetch_secret() {
    local project=$1 ref=$2 name version value
    name=${ref%%#*}
    if [ "$ref" = "$name" ]; then
        version=latest
    else
        version=${ref#*#}
    fi

    if ! value=$(gcloud secrets versions access "$version" \
        --secret="$name" --project="$project" 2>/dev/null); then
        die "secret « $name » (version $version) illisible dans le projet « $project ».
  Vérifiez son nom, le projet, et que votre compte a le rôle roles/secretmanager.secretAccessor."
    fi

    # The .env reader takes one line per variable and knows nothing of escapes,
    # so a value it cannot carry is refused here rather than written broken.
    value=${value%$'\n'}
    case $value in
    *$'\n'*) die "le secret « $name » tient sur plusieurs lignes : .env ne sait pas les relire." ;;
    esac
    printf '%s' "$value"
}

# ---------------------------------------------------------------------------
# Rendering
# ---------------------------------------------------------------------------

# resolve_values fills the two parallel arrays with what the profile declares,
# reading every secret exactly once.
resolve_values() {
    local profile=$1 dry=$2 project name source kind payload
    project=$(profile_project "$profile")
    [ -n "$project" ] || die "le profil « $profile » ne déclare aucun projet Google Cloud."
    case $project in
    CHANGEME-*) die "le profil « $profile » pointe encore sur un placeholder ($project).
  Renseignez son projet et ses secrets dans ${conf_file#"$root"/}." ;;
    esac

    names=()
    values=()
    while IFS=$'\t' read -r name source; do
        [ -n "$name" ] || continue
        kind=${source%%:*}
        payload=${source#*:}
        case $kind in
        value)
            case $payload in
            CHANGEME-*) die "le profil « $profile » déclare $name sur un placeholder ($payload).
  Renseignez la valeur dans ${conf_file#"$root"/}." ;;
            esac
            names+=("$name")
            values+=("$payload")
            ;;
        compose)
            names+=("$name")
            if [ "$dry" = yes ]; then
                values+=("<composé : $payload>")
            else
                values+=("$(compose "$payload")")
            fi
            ;;
        secret)
            case $payload in
            CHANGEME-*) die "le profil « $profile » déclare $name sur un placeholder ($payload).
  Renseignez le nom du secret dans ${conf_file#"$root"/}." ;;
            esac
            names+=("$name")
            if [ "$dry" = yes ]; then
                values+=("<secret $payload, projet $project>")
            else
                values+=("$(fetch_secret "$project" "$payload")")
            fi
            ;;
        *)
            die "source « $source » inconnue pour $name : attendu value:, secret: ou compose:."
            ;;
        esac
    done < <(profile_vars "$profile")

    [ ${#names[@]} -gt 0 ] || die "le profil « $profile » ne déclare aucune variable."
}

render_block() {
    local profile=$1 i
    printf '%s\n' "$BEGIN_MARK"
    printf '# Profil « %s », écrit le %s par scripts/set-env.sh.\n' \
        "$profile" "$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
    printf '# Bloc généré : toute modification faite ici est perdue à la prochaine\n'
    printf '# exécution. Ce qui est hors du bloc est préservé.\n'
    printf 'SECTILE_ENV_PROFILE=%s\n' "$profile"
    for ((i = 0; i < ${#names[@]}; i++)); do
        printf '%s=%s\n' "${names[i]}" "${values[i]}"
    done
    printf '%s\n' "$END_MARK"
}

# splice_block replaces the generated block in place, or appends it when the
# file has none. Nothing outside the markers is touched.
splice_block() {
    local block_file=$1 tmp
    tmp=$(mktemp "${TMPDIR:-/tmp}/sectile-env.XXXXXX")
    # shellcheck disable=SC2064
    trap "rm -f -- '$tmp'" RETURN

    if [ -f "$env_file" ] && grep -qxF -- "$BEGIN_MARK" "$env_file"; then
        awk -v begin="$BEGIN_MARK" -v end="$END_MARK" -v blockfile="$block_file" '
            BEGIN { while ((getline line < blockfile) > 0) block = block line "\n" }
            $0 == begin { printf "%s", block; skipping = 1; next }
            skipping && $0 == end { skipping = 0; next }
            !skipping { print }
        ' "$env_file" > "$tmp"
    else
        # First insertion goes on top, and that position is load-bearing: the
        # server keeps the FIRST definition of a key it reads and skips every
        # later one. A block appended at the end would be silently shadowed by
        # whatever the file already said.
        cat -- "$block_file" > "$tmp"
        if [ -f "$env_file" ] && [ -s "$env_file" ]; then
            printf '\n' >> "$tmp"
            cat -- "$env_file" >> "$tmp"
        fi
    fi

    if [ -f "$env_file" ]; then
        cp -- "$env_file" "$env_file.bak"
    fi
    mv -- "$tmp" "$env_file"
    chmod 600 "$env_file"
    trap - RETURN
}

# warn_shadowed names the variables the generated block shares with a line
# outside it. Before the block, that line wins and the profile is a decoy;
# after it, the line is dead. Neither is what anyone meant.
warn_shadowed() {
    local i begin_line name line_no
    begin_line=$(grep -nxF -- "$BEGIN_MARK" "$env_file" | head -n 1 | cut -d: -f1)
    [ -n "$begin_line" ] || return 0
    local end_line
    end_line=$(grep -nxF -- "$END_MARK" "$env_file" | head -n 1 | cut -d: -f1)
    [ -n "$end_line" ] || return 0

    for ((i = 0; i < ${#names[@]}; i++)); do
        name=${names[i]}
        while read -r line_no; do
            [ -n "$line_no" ] || continue
            if [ "$line_no" -gt "$begin_line" ] && [ "$line_no" -lt "$end_line" ]; then
                continue
            fi
            if [ "$line_no" -lt "$begin_line" ]; then
                printf '  ⚠  %s est aussi défini ligne %s, avant le bloc : c'"'"'est cette ligne qui gagne.\n' \
                    "$name" "$line_no" >&2
            else
                printf '  ⚠  %s est aussi défini ligne %s, après le bloc : cette ligne ne sert à rien.\n' \
                    "$name" "$line_no" >&2
            fi
        done < <(grep -n "^[[:space:]]*\(export[[:space:]]\{1,\}\)\{0,1\}$name=" "$env_file" | cut -d: -f1)
    done
}

active_profile() {
    [ -f "$env_file" ] || return 1
    awk -F= '$1 == "SECTILE_ENV_PROFILE" { value = $2 } END { if (value) print value; else exit 1 }' "$env_file"
}

# ---------------------------------------------------------------------------
# Entry point
# ---------------------------------------------------------------------------

profile=""
assume_yes=no
dry_run=no
export_only=no

while [ $# -gt 0 ]; do
    case $1 in
    -h | --help)
        usage
        exit 0
        ;;
    --status)
        if current=$(active_profile); then
            printf '%s\n' "$current"
        else
            printf 'aucun profil actif dans %s\n' "${env_file#"$root"/}" >&2
            exit 1
        fi
        exit 0
        ;;
    --yes | -y) assume_yes=yes ;;
    --dry-run | -n) dry_run=yes ;;
    --export) export_only=yes ;;
    -*) die "option inconnue : $1" ;;
    *)
        [ -z "$profile" ] || die "un seul profil à la fois (déjà « $profile »)."
        profile=$1
        ;;
    esac
    shift
done

[ -f "$conf_file" ] || die "manifeste introuvable : ${conf_file#"$root"/}
  Partez du modèle : cp scripts/env-profiles.conf.sample scripts/env-profiles.conf"

if [ -z "$profile" ]; then
    usage
    printf '\nProfils déclarés : %s\n' "$(known_profiles | tr '\n' ' ')" >&2
    exit 2
fi

known_profiles | grep -qxF -- "$profile" ||
    die "profil « $profile » inconnu. Déclarés : $(known_profiles | tr '\n' ' ')"

# Pointing a development build at production data is the accident this guard
# exists for, and it is not hypothetical: a build that prunes rows reads the
# store it is handed.
if [ "$profile" = prod ] && [ "$assume_yes" = no ] && [ "$dry_run" = no ]; then
    [ -t 0 ] || die "profil « prod » hors terminal : passez --yes pour l'assumer."
    printf 'Activer le profil PROD dans %s ? [tapez « prod » pour confirmer] ' \
        "${env_file#"$root"/}" >&2
    read -r answer
    [ "$answer" = prod ] || die "annulé."
fi

if [ "$dry_run" = no ] && grep -q '^[[:space:]]*var[[:space:]]\{1,\}'"$profile"'[[:space:]].*secret:' <(strip_comments); then
    require_gcloud
fi

names=()
values=()
resolve_values "$profile" "$dry_run"

if [ "$export_only" = yes ]; then
    for ((i = 0; i < ${#names[@]}; i++)); do
        printf 'export %s=%q\n' "${names[i]}" "${values[i]}"
    done
    printf 'export SECTILE_ENV_PROFILE=%q\n' "$profile"
    exit 0
fi

block_file=$(mktemp "${TMPDIR:-/tmp}/sectile-block.XXXXXX")
trap 'rm -f -- "$block_file"' EXIT
chmod 600 "$block_file"
render_block "$profile" > "$block_file"

if [ "$dry_run" = yes ]; then
    printf 'Bloc qui serait écrit dans %s :\n\n' "${env_file#"$root"/}" >&2
    cat -- "$block_file" >&2
    exit 0
fi

splice_block "$block_file"

printf 'Profil « %s » actif dans %s.\n' "$profile" "${env_file#"$root"/}" >&2
for ((i = 0; i < ${#names[@]}; i++)); do
    note "${names[i]}"
done
warn_shadowed
printf 'Sauvegarde : %s\n' "${env_file#"$root"/}.bak" >&2
printf 'Le serveur relit .env au démarrage : redémarrez-le.\n' >&2
