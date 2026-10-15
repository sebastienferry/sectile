import {SETUP_STEPS,supportedEngines,setupReducer,deploymentSucceeded,startupSetupPrompt} from './engine-setup-state.mjs'

const setupFailure=message=>Object.assign(Error(message),{setupMessage:message})
const names={claude:'Claude',codex:'Codex',agy:'Antigravity'}
const labels={skills:'Compétences Sectile',mcp:'Connexion MCP',permissions:'Autorisations',sdd:'SDD du projet'}
const outcomes={pending:'À configurer',success:'Configuration vérifiée',skipped:'Ignoré à votre demande',unknown:'Vérification indisponible',failed:'Échec'}
const element=(tag,text)=>{const node=document.createElement(tag);if(text)node.textContent=text;return node}
const button=(text,action)=>{const node=element('button',text);node.type='button';node.onclick=action;return node}
const select=(label,options)=>{
 const node=element('select');node.setAttribute('aria-label',label)
 for(const [value,text] of options){const option=element('option',text);option.value=value;node.append(option)}
 return node
}
function sandboxReview(values={}){
 const list=element('dl')
 for(const [key,label] of [['enabled','Sandbox'],['autoAllowBashIfSandboxed','Commandes Bash autorisées dans le sandbox'],['allowUnsandboxedCommands','Commandes hors sandbox autorisées'],['additionalDirectories','Dossiers supplémentaires'],['allowedDomains','Domaines autorisés'],['excludedCommands','Commandes exclues du sandbox'],['allowWrite','Écriture autorisée'],['allow','Règles autorisées'],['deny','Règles refusées']]){
  const value=values[key]
  const description=Array.isArray(value)?value.join(', ')||'Aucune règle ajoutée':value===true?'Oui':value===false?'Non':'Réglage natif conservé'
  list.append(element('dt',label),element('dd',description))
 }
 return list
}
export function engineSetup(api,projects,openSettings){
 const section=element('section');section.className='engine-setup';section.hidden=true
 const title=element('h3','Assistant de configuration du moteur IA');title.tabIndex=-1
 const engine=select('Moteur IA',[]),providerHint=element('p'),notice=element('p');notice.setAttribute('role','status');notice.setAttribute('aria-live','polite')
 const navigation=element('nav');navigation.setAttribute('aria-label','Étapes de configuration')
 const body=element('div'),summary=element('div');summary.setAttribute('role','status')
 let state={provider:'',generation:0,view:null,busy:false,error:''},engines=[],step='skills',closed=false
 let operation=false,returnFocus=null
 const source=select('Projet source des compétences',[['','Choisir un projet…'],...projects.map(project=>[project.id,project.name])])
 const target=select('Projet cible SDD',[['','Choisir un projet…'],...projects.map(project=>[project.id,project.name])])
 const connection=select('Connexion MCP',[['remote','Serveur distant · HTTP'],['local','Agent local · HTTP'],['stdio','Avancé · STDIO']])
 const token=()=>({provider:state.provider,generation:state.generation})
 const dispatch=event=>{state=setupReducer(state,event)}
 const read=async(current=token())=>{
  try{
   const view=await api.setupState(current.provider)
   dispatch({type:'read',view,...current})
   if(current.generation===state.generation&&current.provider===state.provider){const choice=view?.steps?.mcp?.configuration;if(choice)connection.value=choice.transport==='stdio'?'stdio':choice.target==='local'?'local':'remote'}
  }catch(error){dispatch({type:'read',view:null,...current});throw error}
 }
 const execute=async action=>{
  if(operation||!state.view)return
  operation=true;const current=token();dispatch({type:'start',...current});render()
  let failure=''
  try{await action(current);await read(current)}catch(error){failure=error.setupMessage||'L’opération a échoué. Vérifiez la connexion, le dossier du projet et les paramètres du fournisseur, puis réessayez.';try{await read(current)}catch{}}
  finally{operation=false;dispatch({type:'end',error:failure,...current});if(!closed)render()}
 }
 const acknowledge=(which,decision)=>execute(async current=>{
  const result=await api.acknowledgeSetup(current.provider,{step:which,decision,fingerprint:state.view.steps[which].fingerprint})
  dispatch({type:'read',view:result,...current})
 })
 const install=(which,projectSelect)=>execute(async current=>{
  const id=projectSelect.value
  if(!id)throw setupFailure('Choisissez explicitement un projet.')
  const info=await api.project(id)
  if(!info.configured)throw setupFailure('Configurez d’abord le dossier local de ce projet.')
  const result=await api.deployProject(id,which==='skills'?'provider-skills':'framework',current.provider)
  if(!deploymentSucceeded(result,which))throw setupFailure(which==='skills'?'L’installation des compétences a échoué. Réessayez.':'L’installation SDD a échoué. Réessayez.')
 })
 const render=()=>{
  body.replaceChildren();summary.replaceChildren()
  engine.disabled=operation
  providerHint.textContent=state.provider?'Cette configuration est partagée par tous les moteurs '+names[state.provider]+'. Compétences, MCP et autorisations concernent votre utilisateur ; le SDD concerne uniquement le projet choisi.':''
  notice.textContent=(state.busy||operation)?'Opération en cours…':state.error||''
  for(const item of navigation.children){item.disabled=operation;item.setAttribute('aria-current',item.dataset.step===step?'step':'false')}
  if(!state.view){body.append(element('p',engines.length?'Connectez un agent local à jour pour lire la configuration.':'Ajoutez un moteur Claude, Codex ou Antigravity dans les paramètres des moteurs IA.'));return}
  const outcome=state.view.steps[step]
  body.append(element('h4',labels[step]),element('p',outcomes[outcome.status]))
  if(step==='skills'){
   body.append(element('p','Installez les compétences autonomes depuis le projet choisi, ou ignorez cette étape si vous utilisez un plugin. Ignorer ne modifie aucun plugin.'),source,button('Installer les compétences',()=>install('skills',source)),button('Ignorer les compétences',()=>acknowledge('skills','skipped')))
  }else if(step==='mcp'){
   body.append(element('p','Le serveur distant écrit la clé d’association dans la configuration du fournisseur. Le proxy local exige que l’agent reste actif. STDIO utilise le pont existant. La vérification confirme la configuration enregistrée, sans tester une session IA.'),connection,button('Configurer MCP',()=>execute(async current=>{await api.configureMCP(current.provider,{target:connection.value==='local'?'local':'remote',transport:connection.value==='stdio'?'stdio':'http'})})))
   if(state.provider==='claude'&&outcome.configuration?.needsRepair)body.append(button('Réparer explicitement la connexion Claude',()=>execute(async()=>{const info=await api.mcpConfig('claude');if(!info.needsRepair)throw setupFailure('Aucune réparation nécessaire.');await api.configureMCP('claude',{...info.choice,repair:true})})))
  }else if(step==='permissions'){
   const controls=state.view.permissions
   body.append(element('p','Vérifiez les choix actuels. Aucun droit supplémentaire n’est accordé automatiquement. Les réglages s’appliquent aux conversations Sectile ; les règles natives du fournisseur restent en vigueur.'))
   if(state.provider==='claude'){
    const modes=select('Mode d’autorisation Claude',[['default','Demander'],['acceptEdits','Accepter les modifications'],['auto','Automatique'],['plan','Planification']]);modes.value=controls.mode
    body.append(modes,button('Enregistrer le mode Claude',()=>execute(()=>api.setupClaudeMode(modes.value))))
    const sandbox=controls.sandbox
    body.append(element('p',sandbox?.platformSandbox===false?'Le sandbox Claude est indisponible sous Windows ; les règles d’autorisation restent applicables.':'Le sandbox et ses règles sont configurés dans les paramètres Sandbox.'))
    body.append(sandboxReview(sandbox?.claudeSandbox))
    body.append(element('p','Projets couverts : '+(sandbox?.projects?.length?sandbox.projects.map(id=>projects.find(project=>project.id===id)?.name||id).join(', '):'tous les projets')))
    body.append(button('Modifier le sandbox et ses projets',()=>openSettings('Sandbox')))
    const projectsDetails=element('details'),heading=element('summary','Règles propres aux projets');projectsDetails.append(heading)
    for(const [id,rules] of Object.entries(controls.projectSandbox||{})){projectsDetails.append(element('h5',projects.find(project=>project.id===id)?.name||id),sandboxReview(rules))}
    if(!Object.keys(controls.projectSandbox||{}).length)projectsDetails.append(element('p','Aucune règle propre à un projet.'))
    body.append(projectsDetails)
   }else if(state.provider==='codex'){
    const reviewer=select('Validation Codex',[['user','Utilisateur'],['auto_review','Validation automatique']]);reviewer.value=controls.approvalsReviewer
    body.append(reviewer,button('Enregistrer la validation Codex',()=>execute(()=>api.saveCodexSettings({approvalsReviewer:reviewer.value}))))
   }else{
    const modes=select('Mode d’autorisation Antigravity',[['default','Demander'],['accept-edits','Accepter les modifications'],['auto','Automatique'],['plan','Planification']]);modes.value=controls.conversationMode
    body.append(modes,button('Enregistrer le mode Antigravity',()=>execute(()=>api.saveAgySettings({conversationMode:modes.value}))))
   }
   body.append(button('Conserver les autorisations vérifiées',()=>acknowledge('permissions','retained')))
  }else{
   body.append(element('p','Le SDD est facultatif. Installez le framework configuré du projet explicitement choisi ; cela ne modifie pas les autres projets.'),target,button('Installer le SDD',()=>install('sdd',target)),button('Ignorer le SDD',()=>acknowledge('sdd','skipped')))
  }
  for(const control of body.querySelectorAll('button,select'))control.disabled=operation
  for(const which of SETUP_STEPS){const result=state.view.steps[which];const project=projects.find(project=>project.id===result.projectId);summary.append(element('p',labels[which]+' : '+outcomes[result.status]+(result.projectId?' · '+(project?.name||result.projectId):'')))}
  summary.append(element('p',state.view.complete?'Configuration terminée. Redémarrez les moteurs concernés. Aucune session IA réelle n’a été testée.':'Les étapes vérifiées et ignorées sont conservées. Terminez les étapes restantes puis redémarrez le moteur.'))
 }
 for(const which of SETUP_STEPS){const item=button(labels[which],()=>{step=which;render();body.querySelector('h4')?.setAttribute('tabindex','-1');body.querySelector('h4')?.focus()});item.dataset.step=which;navigation.append(item)}
 engine.onchange=async()=>{
  const provider=engines.find(item=>item.id===engine.value)?.provider||''
  dispatch({type:'select',provider});const current=token();render()
  try{await read(current);if(!closed)render()}catch{dispatch({type:'end',error:'Mettez à jour ou reconnectez l’agent local.',...current});if(!closed)render()}
 }
 section.append(title,engine,providerHint,navigation,notice,body,summary,button('Actualiser la configuration',async()=>{if(operation)return;const current=token();try{await read(current)}catch{dispatch({type:'end',error:'Impossible de lire la configuration. Reconnectez l’agent local.',...current})}if(!closed)render()}),button('Fermer l’assistant',()=>{closed=true;section.hidden=true;if(returnFocus?.isConnected)returnFocus.focus()}))
 return {section,open:async()=>{
  returnFocus=document.activeElement;closed=false;section.hidden=false;title.focus()
  const selectedID=engine.value
  dispatch({type:'select',provider:state.provider});render()
  try{
   const [catalogue,currentProjects]=await Promise.all([api.engines(),api.projects()])
   projects=currentProjects
   for(const projectSelect of [source,target]){
    const selectedProject=projectSelect.value
    projectSelect.replaceChildren()
    for(const [value,text] of [['','Choisir un projet…'],...projects.map(project=>[project.id,project.name])]){const option=element('option',text);option.value=value;projectSelect.append(option)}
    if(projects.some(project=>project.id===selectedProject))projectSelect.value=selectedProject
   }
   engines=supportedEngines(catalogue);engine.replaceChildren()
   for(const item of engines){const option=element('option',item.name+' · '+names[item.provider]);option.value=item.id;engine.append(option)}
   if(engines.some(item=>item.id===selectedID))engine.value=selectedID
   else if(engines.some(item=>item.id===catalogue.default))engine.value=catalogue.default
   if(engines.length)await engine.onchange();else render()
  }catch(error){notice.textContent='Connectez un agent local à jour.'}
 }}
}
export function setupStartup(api,open){
 const check=startupSetupPrompt()
 return ()=>check(api,async providers=>{
  if(api.claimSetupPrompt&&!(await api.claimSetupPrompt()))return
  const toast=element('aside');toast.className='engine-setup-toast';toast.setAttribute('role','status')
  toast.append(element('p','Configuration à terminer : '+providers.map(provider=>names[provider]).join(', ')),button('Ouvrir l’assistant',()=>{toast.remove();open()}),button('Ignorer pour ce lancement',()=>toast.remove()))
  document.body.append(toast)
 })
}
