import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {ProjectProfiles} from "../src/components/ProjectProfiles";
import ProjectPrompt from "../src/components/ProjectPrompt";
import * as profiles from "../src/lib/profiles";
import * as chat from "../src/lib/conversation";
vi.mock("../src/lib/profiles",()=>({conversationProfiles:vi.fn(),controlProfiles:vi.fn()}));
vi.mock("../src/lib/conversation",async original=>({...await original<typeof chat>(),providerModels:vi.fn(),createConversation:vi.fn(),controlConversation:vi.fn(),readConversation:vi.fn(),renameConversation:vi.fn()}));
const catalogue={profiles:[{id:"review",name:"Review",provider:"codex",model:"chosen-model",mode:"plan" as const,canDelete:false}],defaultId:"review",inherited:false};
beforeEach(()=>{vi.resetAllMocks();vi.mocked(profiles.conversationProfiles).mockResolvedValue(catalogue);vi.mocked(profiles.controlProfiles).mockResolvedValue(catalogue);vi.mocked(chat.providerModels).mockResolvedValue(["chosen-model"]);vi.mocked(chat.controlConversation).mockResolvedValue({});});
afterEach(cleanup);
test("a project default preloads the new prompt and sends the named profile and options once",async()=>{
 vi.mocked(chat.createConversation).mockResolvedValue({id:"profile-first-turn"});vi.mocked(chat.readConversation).mockResolvedValue({id:"profile-first-turn",busy:false,readOnly:false});
 render(<ProjectPrompt projectId="project" onCreated={()=>{}}/>);
 await screen.findByRole("option",{name:"Codex · Review"});
 expect((screen.getByLabelText("Profil ou moteur") as HTMLSelectElement).value).toBe("profile:review");
 expect((screen.getByLabelText("Mode de conversation") as HTMLSelectElement).value).toBe("plan");
 fireEvent.change(screen.getByLabelText("Prompt libre"),{target:{value:"Review the project"}});fireEvent.click(screen.getByLabelText("Envoyer"));
 await waitFor(()=>expect(chat.controlConversation).toHaveBeenCalledWith("profile-first-turn",{action:"message",message:"Review the project",model:"chosen-model",mode:"plan"}));
 expect(chat.createConversation).toHaveBeenCalledWith("project","codex","review");expect(chat.controlConversation).toHaveBeenCalledTimes(1);
});
test("late defaults preserve choices made while profiles load",async()=>{
 let resolve!:(value:profiles.ProjectProfiles)=>void;vi.mocked(profiles.conversationProfiles).mockReturnValue(new Promise(done=>resolve=done));
 render(<ProjectPrompt projectId="late-project" onCreated={()=>{}}/>);
 fireEvent.change(screen.getByLabelText("Mode de conversation"),{target:{value:"execute"}});resolve(catalogue);
 await screen.findByRole("option",{name:"Codex · Review"});
 expect((screen.getByLabelText("Mode de conversation") as HTMLSelectElement).value).toBe("execute");expect((screen.getByLabelText("Profil ou moteur") as HTMLSelectElement).value).toBe("engine:claude");
});
test("the combined menu switches a saved profile to a standalone engine without carrying profile settings",async()=>{
 vi.mocked(chat.createConversation).mockResolvedValue({id:"standalone-turn"});
 vi.mocked(chat.readConversation).mockResolvedValue({id:"standalone-turn",busy:false,readOnly:false});
 render(<ProjectPrompt projectId="project" onCreated={()=>{}}/>);
 await screen.findByRole("option",{name:"Codex · Review"});
 const choice=screen.getByLabelText("Profil ou moteur") as HTMLSelectElement;
 expect(choice.value).toBe("profile:review");
 fireEvent.change(choice,{target:{value:"engine:claude"}});
 expect(choice.value).toBe("engine:claude");
 expect((screen.getByLabelText("Mode de conversation") as HTMLSelectElement).value).toBe("");
 fireEvent.change(screen.getByLabelText("Prompt libre"),{target:{value:"Explain this project"}});
 fireEvent.click(screen.getByLabelText("Envoyer"));
 await waitFor(()=>expect(chat.createConversation).toHaveBeenCalledWith("project","claude"));
 expect(chat.createConversation).not.toHaveBeenCalledWith("project","claude","review");
});
test("saving a project profile only configures the agent and never starts a conversation",async()=>{
 render(<ProjectProfiles projectId="project"/>);await screen.findByLabelText("Profil par défaut");
 fireEvent.click(screen.getByText("Gérer les profils"));
 fireEvent.click(screen.getByText("Créer un profil"));fireEvent.change(screen.getByLabelText("Nom du profil"),{target:{value:"Implementation"}});
 fireEvent.change(screen.getByLabelText("Mode de conversation"),{target:{value:"execute"}});fireEvent.click(screen.getByText("Enregistrer le profil"));
 await waitFor(()=>expect(profiles.controlProfiles).toHaveBeenCalledWith("project",{action:"save",id:"",name:"Implementation",provider:"claude",model:"",mode:"execute",makeDefault:true}));
 expect(chat.createConversation).not.toHaveBeenCalled();expect(chat.controlConversation).not.toHaveBeenCalled();
});
test("a saved free history can be renamed without sending a provider turn",async()=>{
 const id="rename-free-history";vi.mocked(chat.readConversation).mockResolvedValue({id,title:"Old title",canRename:true,busy:false,readOnly:true,events:[]});vi.mocked(chat.renameConversation).mockResolvedValue({});
 const changed=vi.fn();render(<ProjectPrompt projectId="project" runId={id} onCreated={()=>{}} onTitleChanged={changed}/>);
 fireEvent.click(await screen.findByLabelText("Renommer la conversation"));fireEvent.change(screen.getByLabelText("Nom de la conversation"),{target:{value:"New title"}});fireEvent.click(screen.getByText("Enregistrer le nom"));
 await waitFor(()=>expect(changed).toHaveBeenCalled());expect(chat.renameConversation).toHaveBeenCalledWith(id,"New title");expect(chat.controlConversation).not.toHaveBeenCalled();expect(profiles.conversationProfiles).not.toHaveBeenCalled();
});

test("a profile accepts a custom model identifier without resetting its input",async()=>{
 render(<ProjectProfiles projectId="project"/>);await screen.findByLabelText("Profil par défaut");
 const management=screen.getByText("Gérer les profils").closest("details")!;
 expect(management.open).toBe(false);fireEvent.click(screen.getByText("Gérer les profils"));
 fireEvent.click(screen.getByText("Créer un profil"));
 fireEvent.change(screen.getByLabelText("Nom du profil"),{target:{value:"Custom model"}});
 fireEvent.change(screen.getByLabelText("Modèle"),{target:{value:"__custom__"}});
 fireEvent.change(screen.getByLabelText("Identifiant du modèle"),{target:{value:"gpt-6-luna"}});
 fireEvent.click(screen.getByText("Gérer les profils"));expect(management.open).toBe(false);
 expect(profiles.controlProfiles).not.toHaveBeenCalled();
 fireEvent.click(screen.getByText("Gérer les profils"));
 expect((screen.getByLabelText("Identifiant du modèle") as HTMLInputElement).value).toBe("gpt-6-luna");
 fireEvent.click(screen.getByText("Enregistrer le profil"));
 await waitFor(()=>expect(profiles.controlProfiles).toHaveBeenCalledWith("project",expect.objectContaining({action:"save",model:"gpt-6-luna"})));
});
