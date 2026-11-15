import {useEffect,useRef,useState} from "react";
import {chooseRepositoryFolder,chooseSpecificationFolder,projectContext,projectRepositories,type ProjectContext,type ProjectRepository} from "../lib/workspace";
export function AdvancedFolders({projectId,active,defaultOpen,onChanged}:{projectId:string;active?:boolean;defaultOpen?:boolean;onChanged?:()=>void}){
 const [context,setContext]=useState<ProjectContext>();const [repositories,setRepositories]=useState<ProjectRepository[]>([]);
 const [loaded,setLoaded]=useState(false),[pending,setPending]=useState(false),[error,setError]=useState("");const acting=useRef(false);
 async function refresh(){const [project,repos]=await Promise.all([projectContext(projectId),projectRepositories(projectId)]);setContext(project);setRepositories(Array.isArray(repos)?repos:[]);setLoaded(true);}
 async function act(action:()=>Promise<unknown>){if(acting.current)return;acting.current=true;setPending(true);setError("");try{await action();if(action!==refresh)await refresh();onChanged?.();}catch{setError("Unable to confirm this folder. Check the expected repository and project mapping.");}finally{acting.current=false;setPending(false);}}
 useEffect(()=>{if(active&&!loaded)void act(refresh);},[active,loaded,projectId]);
 return <details className="advanced-folders" open={defaultOpen||undefined} onToggle={e=>{if(e.currentTarget.open&&!loaded)void act(refresh);}}><summary>Repositories and specification folders<small>Map Git repositories and Macro / Issue locations</small></summary><p className="muted">These roles complement attached folders. A declared repository must match its remote; its worktrees and PRs remain managed by Go.</p>
 {error&&<p role="alert" className="error">{error}</p>}
 {context&&([ ["macro","Macro",context.specPath,context.specDefault,context.specKind],["issue","Issue",context.issueSpecPath,context.issueSpecDefault,context.issueSpecKind] ] as const).map(([kind,label,path,fallback,state])=><section key={kind}><h4>{label} specifications</h4><code>{path||fallback||"Working folder not configured"}</code><p className="muted">{path?"Separate folder":"Follows working folder"}{state==="missing"&&" · Folder not found"}</p><button disabled={pending} onClick={()=>void act(()=>chooseSpecificationFolder(projectId,kind))}>Choose {label} folder</button>{path&&<button disabled={pending} onClick={()=>void act(()=>chooseSpecificationFolder(projectId,kind,true))}>Use working folder for {label}</button>}</section>)}
 {repositories.filter(r=>!r.code).map(repo=><section key={repo.identity}><h4>{repo.identity}</h4><code>{repo.path||"Folder not mapped"}</code><button disabled={pending} onClick={()=>void act(()=>chooseRepositoryFolder(projectId,repo.identity))}>Map repository {repo.identity}</button></section>)}
 </details>;
}
