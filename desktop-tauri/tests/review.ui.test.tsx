import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {RunReview} from "../src/components/RunReview";
import * as api from "../src/lib/workspace";
vi.mock("../src/lib/workspace",()=>({runDiff:vi.fn(),editorPreference:vi.fn(),openRunEditor:vi.fn()}));
beforeEach(()=>{vi.resetAllMocks();vi.mocked(api.editorPreference).mockResolvedValue({configured:false,selection:""});vi.mocked(api.runDiff).mockResolvedValue({directory:"/worktree",branch:"feat/test",isClean:false,complete:true,filesChanged:1,additions:1,deletions:0,files:[{path:"file.txt",status:"M",kind:"text",patch:"+<script>malicious()</script>"}]});});afterEach(cleanup);
test("review reads a run folder only on demand and shows untrusted patches as text",async()=>{
 const run={id:"run",projectId:"project",taskId:"task",skill:"implement",status:"completed",createdAt:"2026-10-28",directory:"/worktree",folders:[{path:"/other",name:"other",role:"context"}]};
 const view=render(<RunReview run={run}/>);expect(api.runDiff).not.toHaveBeenCalled();
 const details=view.container.querySelector("details")!;details.open=true;fireEvent(details,new Event("toggle"));
 expect(await screen.findByText("1 fichiers · +1 −0")).toBeTruthy();
 expect(api.runDiff).toHaveBeenCalledWith("run",undefined);expect(view.container.querySelector("script")).toBeNull();
 fireEvent.change(screen.getByLabelText("Dossier de la revue"),{target:{value:"/other"}});
 await waitFor(()=>expect(api.runDiff).toHaveBeenCalledWith("run","/other"));expect(api.openRunEditor).not.toHaveBeenCalled();
});
