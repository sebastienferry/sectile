import {ProviderPermissions} from "./ProviderPermissions";
import {useEffect,useRef,useState} from "react";
import {configureProviderMcp,installProviderSkills,providerReadiness,type Readiness} from "../lib/workspace";

const statusLabel=(status?:string)=>status==="success"?"Configuration vérifiée":status==="skipped"?"Ignoré explicitement":status==="failed"?"Échec":status==="unknown"?"Vérification indisponible":"Non vérifié";
export function ProviderPreparation({provider,projects}:{provider:string;projects:{id:string;name:string}[]}){
 const [view,setView]=useState<Readiness>();
 const [source,setSource]=useState("");
 const [pending,setPending]=useState(false);
 const [replace,setReplace]=useState(false);
 const [error,setError]=useState("");
 const [notice,setNotice]=useState("");
 const mounted=useRef(false),acting=useRef(false);
 async function refresh(){const data=await providerReadiness(provider);if(mounted.current)setView(data);}
 useEffect(()=>{mounted.current=true;void refresh().catch(()=>{if(mounted.current)setError("Préparation Sectile indisponible. Vérifiez les services locaux.");});return()=>{mounted.current=false;};},[provider]);
 async function act(action:()=>Promise<unknown>,message:string){if(acting.current)return;acting.current=true;setPending(true);setError("");setNotice("");try{await action();await refresh();if(mounted.current){setNotice(message);setReplace(false);}}catch{if(mounted.current)setError("L’opération n’a pas pu être confirmée. Vérifiez le projet source et les exécutions ouvertes, puis actualisez avant de réessayer.");}finally{acting.current=false;if(mounted.current)setPending(false);}}
 const isolated=view?.steps?.mcp?.scope==="session";
 const conflict=!!view?.steps?.mcp?.configuration?.present&&!view.steps.mcp.configuration.valid;
 return <details className="provider-preparation"><summary>Skills et connexion MCP Sectile</summary>
 <p>La connexion à la CLI et sa préparation pour les workflows Sectile sont distinctes. {isolated?"Le MCP est fourni uniquement aux sessions de ce compagnon, sans modifier la configuration d’Electron.":"Les actions MCP configurent votre utilisateur pour ce moteur."} Ces actions ne lancent aucune tâche.</p>
 {error&&<p role="alert" className="error">{error}</p>}{notice&&<p role="status">{notice}</p>}
 <dl><dt>Skills</dt><dd>{statusLabel(view?.steps?.skills?.status)}</dd><dt>MCP</dt><dd>{statusLabel(view?.steps?.mcp?.status)}</dd><dt>Autorisations</dt><dd>Réglages actuels conservés · {view?.permissions?.mode ? String(view.permissions.mode):"permissions de la CLI"}</dd></dl>
 <p className="muted">« Non vérifié » ne signifie pas absent : une installation par plugin ou manuelle peut déjà fonctionner. Une configuration MCP enregistrée reste à tester dans une conversation.</p>
 <label>Projet source des skills<select aria-label={`Projet source des skills ${provider}`} disabled={pending} value={source} onChange={e=>setSource(e.target.value)}><option value="">Choisir un projet configuré…</option>{projects.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</select></label>
 <button disabled={pending||!source} onClick={()=>void act(async()=>{const result=await installProviderSkills(provider,source);if(!result?.success||result.skills?.status!=="success")throw new Error("Skill installation not confirmed");},"Skills installées et vérifiées. Les réglages MCP restent indépendants.")}>Installer les skills</button>
 <p className="muted">L’installation nécessite un dépôt configuré et des exécutions arrêtées. Elle ne modifie pas les autorisations de la CLI.</p>
 {conflict&&<label className="profile-checkbox"><input type="checkbox" checked={replace} disabled={pending} onChange={e=>setReplace(e.target.checked)}/> Remplacer l’entrée MCP Sectile existante par celle du compagnon local. L’autre desktop peut utiliser cette entrée.</label>}
 {isolated?<p className="muted">Connexion MCP isolée active pour les prochains lancements Claude Code et Codex. Les conversations Codex déjà ouvertes utilisent leur configuration jusqu’au redémarrage de leur processus.</p>:<button disabled={pending||!view||(conflict&&!replace)} onClick={()=>void act(()=>configureProviderMcp(provider,replace),"MCP enregistré vers l’agent local. Vérifiez ensuite son accès dans une conversation.")}>Configurer le MCP local</button>}
 <button disabled={pending} onClick={()=>void act(refresh,"État de préparation actualisé.")}>Vérifier la préparation</button>
 <ProviderPermissions provider={provider} view={view} onSaved={refresh}/>
 </details>;
}
