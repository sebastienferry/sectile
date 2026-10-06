// The presets of the workstation Claude settings (#745): fixed sets of entries
// for its five lists, one per toolchain plus a Common set, a deny set and a
// set of commands run outside the sandbox. A preset is copied into the lists
// when applied: the entries become ordinary ones, and nothing records which
// preset added them, so an update of this catalogue never changes the lists an
// owner already has.
//
// Rules are written as Claude Code documents them, `Bash(<command> *)`, where
// the trailing ` *` also matches the bare command. The commands Claude Code
// already runs without asking (ls, cat, grep, find, read-only git) get no
// allow rule. A cache path is listed under both its macOS and Linux names.

export const PRESETS=[
 {id:'common',name:'Common',description:'Everyday git commands and the GitHub and GitLab hosts.',
  allow:['Bash(git add *)','Bash(git commit *)','Bash(git switch *)','Bash(git fetch *)','Bash(git pull *)','Bash(gh pr view *)','Bash(gh pr checks *)','Bash(gh issue view *)'],
  deny:[],
  allowedDomains:['github.com','api.github.com','codeload.github.com','*.githubusercontent.com','gitlab.com'],
  allowWrite:[],
  excludedCommands:[]},
 {id:'go',name:'Go',description:'Build, test and lint Go code, with the module proxy and the build caches.',
  allow:['Bash(go build *)','Bash(go test *)','Bash(go vet *)','Bash(go mod *)','Bash(gofmt *)','Bash(golangci-lint *)'],
  deny:[],
  allowedDomains:['proxy.golang.org','sum.golang.org'],
  allowWrite:['~/Library/Caches/go-build','~/.cache/go-build','~/go/pkg/mod'],
  excludedCommands:[]},
 {id:'node',name:'Node / TypeScript',description:'Install, build and test with npm, pnpm or yarn, with their registries and caches.',
  allow:['Bash(npm ci)','Bash(npm install *)','Bash(npm run *)','Bash(npm test *)','Bash(npx tsc *)','Bash(node --test *)',
   'Bash(pnpm install *)','Bash(pnpm run *)','Bash(pnpm test *)','Bash(yarn install *)','Bash(yarn run *)','Bash(yarn test *)'],
  deny:[],
  allowedDomains:['registry.npmjs.org','registry.yarnpkg.com'],
  allowWrite:['~/.npm','~/Library/Caches/Yarn','~/.cache/yarn','~/Library/pnpm','~/.local/share/pnpm'],
  excludedCommands:[]},
 {id:'python',name:'Python',description:'Install, test and lint with pip, uv, pytest, ruff and mypy, with PyPI and the caches.',
  allow:['Bash(pytest *)','Bash(python -m pytest *)','Bash(python3 -m pytest *)','Bash(pip install *)','Bash(uv sync *)','Bash(uv pip install *)','Bash(ruff *)','Bash(mypy *)'],
  deny:[],
  allowedDomains:['pypi.org','files.pythonhosted.org'],
  allowWrite:['~/Library/Caches/pip','~/.cache/pip','~/.cache/uv'],
  excludedCommands:[]},
 {id:'jvm',name:'Java / Kotlin',description:'Compile, test and package with Maven or Gradle, with their repositories and caches.',
  allow:['Bash(mvn compile *)','Bash(mvn test *)','Bash(mvn package *)','Bash(mvn verify *)',
   'Bash(./mvnw compile *)','Bash(./mvnw test *)','Bash(./mvnw package *)','Bash(./mvnw verify *)',
   'Bash(gradle build *)','Bash(gradle test *)','Bash(gradle check *)','Bash(./gradlew build *)','Bash(./gradlew test *)','Bash(./gradlew check *)'],
  deny:[],
  allowedDomains:['repo.maven.apache.org','repo1.maven.org','plugins.gradle.org','services.gradle.org','downloads.gradle.org'],
  allowWrite:['~/.m2','~/.gradle'],
  excludedCommands:[]},
 {id:'rust',name:'Rust',description:'Build, test, lint and format with cargo, with crates.io and the cargo home.',
  allow:['Bash(cargo build *)','Bash(cargo test *)','Bash(cargo check *)','Bash(cargo clippy *)','Bash(cargo fmt *)'],
  deny:[],
  allowedDomains:['crates.io','index.crates.io','static.crates.io'],
  allowWrite:['~/.cargo'],
  excludedCommands:[]},
 {id:'dotnet',name:'.NET',description:'Restore, build, test and format with dotnet, with NuGet and its package cache.',
  allow:['Bash(dotnet build *)','Bash(dotnet test *)','Bash(dotnet restore *)','Bash(dotnet format *)'],
  deny:[],
  allowedDomains:['api.nuget.org'],
  allowWrite:['~/.nuget/packages'],
  excludedCommands:[]},
 {id:'infra',name:'Infrastructure (read-only)',description:'Plan and inspect with Terraform, Terragrunt, kubectl and Helm, without changing anything.',
  allow:['Bash(terraform fmt *)','Bash(terraform validate *)','Bash(terraform init *)','Bash(terraform plan *)',
   'Bash(terragrunt validate *)','Bash(terragrunt plan *)','Bash(kubectl get *)','Bash(kubectl describe *)','Bash(kubectl logs *)',
   'Bash(helm template *)','Bash(helm lint *)'],
  deny:[],
  allowedDomains:['registry.terraform.io','releases.hashicorp.com'],
  allowWrite:['~/.terraform.d'],
  excludedCommands:[]},
 {id:'terragrunt',name:'Terragrunt',description:'Format, validate, plan and inspect Terragrunt stacks, one unit or all of them, with the provider cache.',
  allow:['Bash(terragrunt init *)','Bash(terragrunt validate *)','Bash(terragrunt plan *)','Bash(terragrunt hclfmt *)','Bash(terragrunt hcl fmt *)',
   'Bash(terragrunt output *)','Bash(terragrunt show *)','Bash(terragrunt render-json *)','Bash(terragrunt graph-dependencies *)',
   'Bash(terragrunt run-all validate *)','Bash(terragrunt run-all plan *)','Bash(terragrunt run --all validate *)','Bash(terragrunt run --all plan *)'],
  deny:[],
  allowedDomains:['registry.terraform.io','releases.hashicorp.com'],
  allowWrite:['~/.terraform.d','~/.cache/terragrunt'],
  excludedCommands:[]},
 {id:'gcloud',name:'Google Cloud (read-only)',description:'List and describe projects, clusters, services and logs with gcloud, without changing anything or reading a secret value.',
  allow:['Bash(gcloud config list *)','Bash(gcloud config get-value *)','Bash(gcloud auth list *)',
   'Bash(gcloud projects list *)','Bash(gcloud projects describe *)',
   'Bash(gcloud container clusters list *)','Bash(gcloud container clusters describe *)',
   'Bash(gcloud compute instances list *)','Bash(gcloud compute networks list *)',
   'Bash(gcloud run services list *)','Bash(gcloud run services describe *)',
   'Bash(gcloud sql instances list *)','Bash(gcloud pubsub topics list *)','Bash(gcloud artifacts repositories list *)',
   'Bash(gcloud iam service-accounts list *)','Bash(gcloud secrets list *)','Bash(gcloud logging read *)'],
  deny:[],
  allowedDomains:['oauth2.googleapis.com','cloudresourcemanager.googleapis.com','container.googleapis.com','compute.googleapis.com','run.googleapis.com',
   'sqladmin.googleapis.com','pubsub.googleapis.com','artifactregistry.googleapis.com','iam.googleapis.com','secretmanager.googleapis.com','logging.googleapis.com'],
  allowWrite:['~/.config/gcloud'],
  excludedCommands:[]},
 {id:'glab',name:'GitLab CLI',description:'Read merge requests, issues and pipelines with glab.',
  allow:['Bash(glab mr view *)','Bash(glab mr list *)','Bash(glab mr diff *)','Bash(glab issue view *)','Bash(glab issue list *)',
   'Bash(glab ci status *)','Bash(glab ci list *)','Bash(glab ci trace *)','Bash(glab repo view *)'],
  deny:[],
  allowedDomains:['gitlab.com'],
  allowWrite:['~/.config/glab-cli'],
  excludedCommands:[]},
 // The commands that need SSH keys or the system trust store, which the macOS
 // sandbox blocks (#764). Local git stays sandboxed: only what reaches a
 // remote leaves it.
 {id:'outside-sandbox',name:'Outside the sandbox (git, gh, glab)',
  description:'Run git fetch, pull, push, clone and ls-remote, gh and glab outside the sandbox, so SSH keys and the system certificates work. They run with full access and still follow the allow and deny rules: git push keeps asking unless an allow rule covers it.',
  allow:[],
  deny:[],
  allowedDomains:[],
  allowWrite:[],
  excludedCommands:['git fetch *','git pull *','git push *','git clone *','git ls-remote *','gh *','glab *']},
 {id:'dangerous',name:'Dangerous actions',
  description:'Never apply infrastructure, delete cloud resources, force-push, publish, use sudo or read secrets. A deny rule matches the command as Claude Code writes it: the same program run another way, such as through sh -c or a script, is not matched, so this is a guardrail, not a security boundary.',
  allow:[],
  deny:['Bash(terraform apply *)','Bash(terraform destroy *)','Bash(terraform import *)','Bash(terraform state rm *)','Bash(terraform state mv *)','Bash(terraform state push *)',
   'Bash(terragrunt apply *)','Bash(terragrunt destroy *)','Bash(terragrunt run-all apply *)','Bash(terragrunt run-all destroy *)',
   'Bash(terragrunt run --all apply *)','Bash(terragrunt run --all destroy *)',
   'Bash(kubectl apply *)','Bash(kubectl delete *)','Bash(kubectl patch *)','Bash(kubectl replace *)',
   'Bash(helm install *)','Bash(helm upgrade *)','Bash(helm uninstall *)','Bash(helm rollback *)',
   'Bash(gcloud * delete *)','Bash(gcloud secrets versions access *)','Bash(aws * delete-*)','Bash(aws s3 rm *)','Bash(aws ec2 terminate-instances *)','Bash(az * delete *)',
   'Bash(git push --force)','Bash(git push --force *)','Bash(git push -f *)','Bash(git push * --force)','Bash(git push * --force *)','Bash(git push * -f)','Bash(git push * -f *)',
   'Bash(npm publish *)','Bash(pnpm publish *)','Bash(yarn publish *)','Bash(cargo publish *)','Bash(twine upload *)',
   'Bash(mvn deploy *)','Bash(./mvnw deploy *)','Bash(docker push *)','Bash(gh release create *)','Bash(glab release create *)','Bash(glab repo delete *)',
   'Bash(sudo *)',
   'Read(./**/.env)','Read(./**/.env.local)','Read(./**/.env.*.local)','Read(./**/.env.production)','Read(./**/*.tfstate)',
   'Read(~/.ssh/**)','Read(~/.aws/**)','Read(~/.config/gcloud/**)'],
  allowedDomains:[],
  allowWrite:[],
  excludedCommands:[]},
]

// The presets "Apply recommended" applies, in order, on an empty workstation.
export const RECOMMENDED=['common','dangerous']

const LISTS=['allowedDomains','excludedCommands','allowWrite','allow','deny']

// applicableLists names the lists this platform applies: the rules alone on
// Windows, where Claude Code's sandbox does not run (#700).
export function applicableLists(platformSandbox){return platformSandbox?LISTS:['allow','deny']}

// presetHasEntries says whether the preset has anything to apply here.
export function presetHasEntries(preset,platformSandbox){
 return applicableLists(platformSandbox).some(list=>preset[list].length>0)
}

// presetApplied says whether every entry of the preset this platform applies
// is in the lists. A preset with nothing to apply here is never applied.
export function presetApplied(values,preset,platformSandbox){
 if(!presetHasEntries(preset,platformSandbox))return false
 return applicableLists(platformSandbox).every(list=>preset[list].every(entry=>values[list].includes(entry)))
}

// applyPreset appends the preset's missing entries, in catalogue order, and
// leaves the entries already there as they are.
export function applyPreset(values,preset,platformSandbox){
 const next={...values}
 for(const list of applicableLists(platformSandbox))next[list]=[...values[list],...preset[list].filter(entry=>!values[list].includes(entry))]
 return next
}

// removePreset takes out the preset's entries, except those another applied
// preset also holds. An entry typed by hand goes too: the lists do not record
// where an entry came from.
export function removePreset(values,preset,platformSandbox,presets=PRESETS){
 const others=presets.filter(other=>other.id!==preset.id&&presetApplied(values,other,platformSandbox))
 const next={...values}
 for(const list of applicableLists(platformSandbox)){
  const kept=new Set(others.flatMap(other=>other[list]))
  next[list]=values[list].filter(entry=>!preset[list].includes(entry)||kept.has(entry))
 }
 return next
}

// listsEmpty says whether the five lists are empty, whatever the state.
export function listsEmpty(values){return LISTS.every(list=>values[list].length===0)}
