import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import ProjectPrompt from "../src/components/ProjectPrompt";
import * as api from "../src/lib/conversation";
vi.mock("../src/components/FolderContext",()=>({PromptFolders:()=>null}));
vi.mock("../src/lib/conversation",async original=>({...await original<typeof api>(),providerModels:vi.fn(),readConversation:vi.fn(),controlConversation:vi.fn()}));
beforeEach(()=>{vi.resetAllMocks();vi.mocked(api.providerModels).mockResolvedValue([]);vi.mocked(api.controlConversation).mockResolvedValue({accepted:true});});afterEach(cleanup);
test("an idle conversation follows a pending MCP check without sending a provider message",async()=>{
 const base={id:"mcp-check-idle",provider:"claude",busy:false,readOnly:false,events:[],sectileMcp:{status:"unknown"}};
 vi.mocked(api.readConversation).mockResolvedValueOnce(base).mockResolvedValueOnce({...base,checkingMcp:true}).mockResolvedValue({...base,checkingMcp:false,sectileMcp:{status:"connected"}});
 render(<ProjectPrompt projectId="project" runId={base.id} onCreated={()=>{}}/>);
 const check=screen.getByRole("button",{name:"Vérifier le MCP de cette conversation"});await waitFor(()=>expect(check.hasAttribute("disabled")).toBe(false));
 fireEvent.click(check);
 await waitFor(()=>expect(check.getAttribute("title")).toBe("MCP du moteur · Vérification en cours…"));
 await waitFor(()=>expect(check.getAttribute("title")).toBe("MCP du moteur · Connecté"),{timeout:2500});
 expect(api.controlConversation).toHaveBeenCalledExactlyOnceWith(base.id,{action:"checkMcp"});
 expect(api.readConversation).toHaveBeenCalledTimes(3);
});
test("a restored Codex conversation explains when its MCP check can run",async()=>{
 const base={id:"mcp-restored-codex",provider:"codex",busy:false,readOnly:false,events:[],sectileMcp:{status:"unknown"}};
 vi.mocked(api.readConversation).mockResolvedValue(base);
 render(<ProjectPrompt projectId="project" runId={base.id} onCreated={()=>{}}/>);
 const check=await screen.findByRole("button",{name:"Vérifier le MCP de cette conversation"});
 await waitFor(()=>expect(check.hasAttribute("disabled")).toBe(false));
 fireEvent.click(check);
 expect(await screen.findByText(/Codex vérifiera le MCP au prochain message/)).toBeTruthy();
 expect(api.controlConversation).toHaveBeenCalledExactlyOnceWith(base.id,{action:"checkMcp"});
});
test("an unbound saved conversation requires and records explicit connection adoption",async()=>{
 const base={id:"legacy-conversation",provider:"claude",busy:false,readOnly:true,connectionId:"",canAdoptConnection:true,events:[]};
 vi.mocked(api.readConversation).mockResolvedValueOnce(base).mockResolvedValue({...base,readOnly:false,connectionId:"11111111-1111-4111-8111-111111111111",canAdoptConnection:false});
 render(<ProjectPrompt projectId="project" runId={base.id} onCreated={()=>{}}/>);
 const adopt=await screen.findByRole("button",{name:"Associer à cette connexion"});
 await waitFor(()=>expect(adopt.hasAttribute("disabled")).toBe(false));
 fireEvent.click(adopt);
 await waitFor(()=>expect(api.controlConversation).toHaveBeenCalledExactlyOnceWith(base.id,{action:"adoptConnection"}));
 await waitFor(()=>expect(screen.getByRole("status").textContent).toBe("Prêt"));
});
