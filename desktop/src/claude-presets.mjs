// The presets of the workstation Claude settings (#745): fixed sets of entries
// for its four lists, one per toolchain plus a Common set and a deny set. A
// preset is copied into the lists when applied: the entries become ordinary
// ones, and nothing records which preset added them, so an update of this
// catalogue never changes the lists an owner already has.
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
  allowWrite:[]},
 {id:'go',name:'Go',description:'Build, test and lint Go code, with the module proxy and the build caches.',
  allow:['Bash(go build *)','Bash(go test *)','Bash(go vet *)','Bash(go mod *)','Bash(gofmt *)','Bash(golangci-lint *)'],
  deny:[],
  allowedDomains:['proxy.golang.org','sum.golang.org'],
  allowWrite:['~/Library/Caches/go-build','~/.cache/go-build','~/go/pkg/mod']},
 {id:'node',name:'Node / TypeScript',description:'Install, build and test with npm, pnpm or yarn, with their registries and caches.',
  allow:['Bash(npm ci)','Bash(npm install *)','Bash(npm run *)','Bash(npm test *)','Bash(npx tsc *)','Bash(node --test *)',
   'Bash(pnpm install *)','Bash(pnpm run *)','Bash(pnpm test *)','Bash(yarn install *)','Bash(yarn run *)','Bash(yarn test *)'],
  deny:[],
  allowedDomains:['registry.npmjs.org','registry.yarnpkg.com'],
  allowWrite:['~/.npm','~/Library/Caches/Yarn','~/.cache/yarn','~/Library/pnpm','~/.local/share/pnpm']},
 {id:'python',name:'Python',description:'Install, test and lint with pip, uv, pytest, ruff and mypy, with PyPI and the caches.',
  allow:['Bash(pytest *)','Bash(python -m pytest *)','Bash(python3 -m pytest *)','Bash(pip install *)','Bash(uv sync *)','Bash(uv pip install *)','Bash(ruff *)','Bash(mypy *)'],
  deny:[],
  allowedDomains:['pypi.org','files.pythonhosted.org'],
  allowWrite:['~/Library/Caches/pip','~/.cache/pip','~/.cache/uv']},
 {id:'jvm',name:'Java / Kotlin',description:'Compile, test and package with Maven or Gradle, with their repositories and caches.',
  allow:['Bash(mvn compile *)','Bash(mvn test *)','Bash(mvn package *)','Bash(mvn verify *)',
   'Bash(./mvnw compile *)','Bash(./mvnw test *)','Bash(./mvnw package *)','Bash(./mvnw verify *)',
   'Bash(gradle build *)','Bash(gradle test *)','Bash(gradle check *)','Bash(./gradlew build *)','Bash(./gradlew test *)','Bash(./gradlew check *)'],
  deny:[],
  allowedDomains:['repo.maven.apache.org','repo1.maven.org','plugins.gradle.org','services.gradle.org','downloads.gradle.org'],
  allowWrite:['~/.m2','~/.gradle']},
 {id:'rust',name:'Rust',description:'Build, test, lint and format with cargo, with crates.io and the cargo home.',
  allow:['Bash(cargo build *)','Bash(cargo test *)','Bash(cargo check *)','Bash(cargo clippy *)','Bash(cargo fmt *)'],
  deny:[],
  allowedDomains:['crates.io','index.crates.io','static.crates.io'],
  allowWrite:['~/.cargo']},
 {id:'dotnet',name:'.NET',description:'Restore, build, test and format with dotnet, with NuGet and its package cache.',
  allow:['Bash(dotnet build *)','Bash(dotnet test *)','Bash(dotnet restore *)','Bash(dotnet format *)'],
  deny:[],
  allowedDomains:['api.nuget.org'],
  allowWrite:['~/.nuget/packages']},
 {id:'infra',name:'Infrastructure (read-only)',description:'Plan and inspect with Terraform, Terragrunt, kubectl and Helm, without changing anything.',
  allow:['Bash(terraform fmt *)','Bash(terraform validate *)','Bash(terraform init *)','Bash(terraform plan *)',
   'Bash(terragrunt validate *)','Bash(terragrunt plan *)','Bash(kubectl get *)','Bash(kubectl describe *)','Bash(kubectl logs *)',
   'Bash(helm template *)','Bash(helm lint *)'],
  deny:[],
  allowedDomains:['registry.terraform.io','releases.hashicorp.com'],
  allowWrite:['~/.terraform.d']},
 {id:'dangerous',name:'Dangerous actions',
  description:'Never apply infrastructure, delete cloud resources, force-push, publish, use sudo or read secrets. A deny rule matches the command as Claude Code writes it: the same program run another way, such as through sh -c or a script, is not matched, so this is a guardrail, not a security boundary.',
  allow:[],
  deny:['Bash(terraform apply *)','Bash(terraform destroy *)','Bash(terraform import *)','Bash(terraform state rm *)','Bash(terraform state mv *)','Bash(terraform state push *)',
   'Bash(terragrunt apply *)','Bash(terragrunt destroy *)','Bash(terragrunt run-all apply *)','Bash(terragrunt run-all destroy *)',
   'Bash(kubectl apply *)','Bash(kubectl delete *)','Bash(kubectl patch *)','Bash(kubectl replace *)',
   'Bash(helm install *)','Bash(helm upgrade *)','Bash(helm uninstall *)','Bash(helm rollback *)',
   'Bash(gcloud * delete *)','Bash(aws * delete-*)','Bash(aws s3 rm *)','Bash(aws ec2 terminate-instances *)','Bash(az * delete *)',
   'Bash(git push --force*)','Bash(git push -f *)','Bash(git push * --force*)','Bash(git push * -f)','Bash(git push * -f *)',
   'Bash(npm publish *)','Bash(pnpm publish *)','Bash(yarn publish *)','Bash(cargo publish *)','Bash(twine upload *)',
   'Bash(mvn deploy *)','Bash(./mvnw deploy *)','Bash(docker push *)','Bash(gh release create *)',
   'Bash(sudo *)',
   'Read(./**/.env)','Read(./**/.env.local)','Read(./**/.env.*.local)','Read(./**/.env.production)','Read(./**/*.tfstate)',
   'Read(~/.ssh/**)','Read(~/.aws/**)','Read(~/.config/gcloud/**)'],
  allowedDomains:[],
  allowWrite:[]},
]

// The presets "Apply recommended" applies, in order, on an empty workstation.
export const RECOMMENDED=['common','dangerous']

const LISTS=['allowedDomains','allowWrite','allow','deny']

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

// listsEmpty says whether the four lists are empty, whatever the state.
export function listsEmpty(values){return LISTS.every(list=>values[list].length===0)}
