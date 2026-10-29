import {useRef,useState} from "react";
import type {Run} from "../domain/execution";
import {editorPreference,openRunEditor,runDiff,type RunDiff} from "../lib/workspace";
export function RunReview({run}:{run:Run}){
 const [folder,setFolder]=useState("");const [diff,setDiff]=useState<RunDiff>();const [error,setError]=useState("");const [pending,setPending]=useState(false);
 const [editor,setEditor]=useState("");const [editorReady,setEditorReady]=useState(false);const [opened,setOpened]=useState(false);
 const request=useRef(0);
 const folders=[...(run.directory?[{path:run.directory,name:"Dossier de travail"}]:[]),...(run.folders??[])].filter((f,i,all)=>all.findIndex(other=>other.path===f.path)===i);
 async function refresh(path=folder){const seq=++request.current;setPending(true);setError("");setDiff(undefined);try{const data=await runDiff(run.id,path||undefined);if(seq===request.current)setDiff(data);}catch{if(seq===request.current)setError("Revue indisponible : vérifiez le checkout Git de cette exécution. Un dossier simple ou un worktree supprimé n’offre pas de diff.");}finally{if(seq===request.current)setPending(false);}}
 return <details className="run-review" onToggle={e=>{if(e.currentTarget.open&&!opened){setOpened(true);void refresh();void editorPreference().then(view=>{setEditor(view.selection);setEditorReady(view.configured);}).catch(()=>{});}}}><summary>Revue des changements</summary><p className="muted">Lecture des modifications du dossier de cette exécution, sans modifier les fichiers.</p>
 <label>Dossier à examiner<select aria-label="Dossier de la revue" value={folder} onChange={e=>{setFolder(e.target.value);void refresh(e.target.value);}}><option value="">Dossier de travail de l’exécution</option>{folders.filter(f=>f.path!==run.directory).map(f=><option value={f.path} key={f.path}>{f.name} · {f.path}</option>)}</select></label>
 <button disabled={pending} onClick={()=>void refresh()}>Actualiser les changements</button>
 <details><summary>Éditeur du poste</summary><label>Éditeur<select aria-label="Éditeur du poste" value={editor} onChange={e=>setEditor(e.target.value)}><option value="">{editorReady?"Configuration existante":"Choisir un éditeur"}</option><option value="code">VS Code</option><option value="cursor">Cursor</option><option value="zed">Zed</option></select></label><button disabled={!editor||pending} onClick={()=>{setPending(true);void editorPreference(editor).then(view=>setEditorReady(view.configured)).catch(()=>setError("Impossible d’enregistrer l’éditeur.")).finally(()=>setPending(false));}}>Enregistrer cet éditeur</button></details>
 <button disabled={!editorReady||pending} onClick={()=>void openRunEditor(run.id,folder||undefined).catch(()=>setError("Impossible d’ouvrir l’éditeur. Vérifiez son installation et le dossier de l’exécution."))}>Ouvrir dans l’éditeur</button>
 {pending&&<p role="status">Lecture des changements…</p>}{error&&<p role="alert" className="error">{error}</p>}
 {diff&&<><p>{diff.isClean?"Aucune modification":`${diff.filesChanged} fichiers · +${diff.additions} −${diff.deletions}`}{!diff.complete&&" · Aperçu partiel"}</p>{diff.warnings?.map((w,i)=><p key={i}>{w.message}</p>)}{diff.files?.map(file=><details key={file.path} className="diff-file"><summary>{file.path} · {file.status}</summary>{file.omittedReason?<p>{file.omittedReason}</p>:<pre><code>{file.patch?.split("\n").map((line,i)=><span key={i} className={line.startsWith("+")?"diff-added":line.startsWith("-")?"diff-removed":""}>{line}{"\n"}</span>)}</code></pre>}</details>)}</>}
 </details>;
}
