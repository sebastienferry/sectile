import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import StartOverDialog from "../src/components/StartOverDialog";
import ProjectPrompt from "../src/components/ProjectPrompt";
import * as chat from "../src/lib/conversation";
import * as controller from "../src/lib/conversation-controller";
import * as profiles from "../src/lib/profiles";
import type {Run} from "../src/domain/execution";
vi.mock("../src/lib/profiles",()=>({conversationProfiles:vi.fn()}));
vi.mock("../src/lib/folders",()=>({projectFolders:vi.fn(async()=>[]),attachFolder:vi.fn()}));
vi.mock("../src/lib/workspace",()=>({projectContext:vi.fn(async()=>({policy:{},skills:[]})),projectRepositories:vi.fn(async()=>[])}));
vi.mock("../src/lib/conversation",async original=>({...await original<typeof chat>(),providerModels:vi.fn(async()=>[]),createConversation:vi.fn(),readConversation:vi.fn(),controlConversation:vi.fn(),formerContextPreview:vi.fn()}));
const source:Run={id:"source",projectId:"project",taskId:"",skill:"",status:"completed",createdAt:"2026-11-01T00:00:00Z",conversation:true,provider:"codex",title:"Cache design"};
const send=vi.fn(async()=>true);
beforeEach(()=>{
 vi.resetAllMocks();
 vi.mocked(profiles.conversationProfiles).mockResolvedValue({profiles:[],defaultId:"",inherited:true});
 vi.mocked(chat.formerContextPreview).mockResolvedValue({sourceId:"source",sourceTitle:"Cache design",messages:12,omitted:3,toolsOmitted:true,characters:99000});
 vi.mocked(chat.createConversation).mockResolvedValue({id:"fresh"});
 vi.mocked(chat.readConversation).mockResolvedValue({id:"source",busy:false,readOnly:true});
 vi.spyOn(controller,"conversationFor").mockReturnValue({send} as unknown as controller.ConversationController);
});
afterEach(cleanup);

function dialog(overrides:Partial<Parameters<typeof StartOverDialog>[0]>={}){
 const props={projectId:"project",source,agentAvailable:true,skills:[],skillUnavailable:"Skills need a conversation linked to a ticket.",onConversationCreated:vi.fn(),onClose:vi.fn(),...overrides};
 render(<StartOverDialog {...props}/>);
 return props;
}

test("a new conversation starts with the archived one as context on the source engine",async()=>{
 const props=dialog();
 await screen.findByText(/12 messages of this conversation will be given as former context\. 3 earlier messages are left out to fit the context limit\. Tool calls/);
 expect((screen.getByLabelText("Profile or engine") as HTMLSelectElement).value).toBe("engine:codex");
 expect(screen.getByRole("button",{name:"Start"}).hasAttribute("disabled")).toBe(true);
 fireEvent.change(screen.getByLabelText("New instructions"),{target:{value:"Add metrics"}});
 fireEvent.click(screen.getByRole("button",{name:"Start"}));
 await waitFor(()=>expect(chat.createConversation).toHaveBeenCalledWith("project","codex",undefined,"source"));
 expect(send).toHaveBeenCalledWith("Add metrics",true,{});
 expect(props.onConversationCreated).toHaveBeenCalledWith("fresh");
});

test("a conversation without a ticket offers no skill and says why",async()=>{
 dialog();
 await screen.findByText(/messages of this conversation/);
 expect((screen.getByRole("radio",{name:"Skill"}) as HTMLInputElement).disabled).toBe(true);
 screen.getByText("Skills need a conversation linked to a ticket.");
});

test("a skill starts on the source's ticket with the new instructions",async()=>{
 const onStartSkill=vi.fn(async()=>{});
 const props=dialog({source:{...source,taskId:"task"},skills:[{id:"clarify",label:"Clarify"},{id:"specify",label:"Specify"}],skillUnavailable:undefined,onStartSkill});
 await screen.findByText(/messages of this conversation/);
 fireEvent.click(screen.getByRole("radio",{name:"Skill"}));
 fireEvent.change(screen.getByRole("combobox",{name:"Skill"}),{target:{value:"specify"}});
 fireEvent.change(screen.getByLabelText("New instructions"),{target:{value:"Write the plan"}});
 fireEvent.click(screen.getByRole("button",{name:"Start"}));
 await waitFor(()=>expect(onStartSkill).toHaveBeenCalledWith("specify","Write the plan"));
 expect(chat.createConversation).not.toHaveBeenCalled();
 expect(props.onClose).toHaveBeenCalled();
});

test("a failed launch keeps the dialog and the typed instructions",async()=>{
 vi.mocked(chat.createConversation).mockRejectedValue("This conversation has no message to start from.");
 const props=dialog();
 await screen.findByText(/messages of this conversation/);
 fireEvent.change(screen.getByLabelText("New instructions"),{target:{value:"Keep me"}});
 fireEvent.click(screen.getByRole("button",{name:"Start"}));
 await screen.findByRole("alert");
 screen.getByText("This conversation has no message to start from.");
 expect((screen.getByLabelText("New instructions") as HTMLTextAreaElement).value).toBe("Keep me");
 expect(props.onClose).not.toHaveBeenCalled();
 expect(send).not.toHaveBeenCalled();
});

test("unavailable history and a disconnected agent block the start",async()=>{
 vi.mocked(chat.formerContextPreview).mockRejectedValue("This conversation's history is not available on this workstation.");
 dialog();
 await screen.findByText("This conversation's history is not available on this workstation.");
 fireEvent.change(screen.getByLabelText("New instructions"),{target:{value:"Try"}});
 expect(screen.getByRole("button",{name:"Start"}).hasAttribute("disabled")).toBe(true);
 cleanup();
 vi.mocked(chat.formerContextPreview).mockResolvedValue({sourceId:"source",sourceTitle:"",messages:1,omitted:0,toolsOmitted:false,characters:5});
 dialog({agentAvailable:false});
 screen.getByText("Agent unavailable. Reconnect to start over.");
 fireEvent.change(screen.getByLabelText("New instructions"),{target:{value:"Try"}});
 expect(screen.getByRole("button",{name:"Start"}).hasAttribute("disabled")).toBe(true);
});

test("Start over is offered on archived conversations only, and a new session links its source",async()=>{
 const onStartOver=vi.fn();
 const view=render(<ProjectPrompt projectId="project" runId="source" run={source} archived={false} onStartOver={onStartOver} onCreated={()=>{}}/>);
 await waitFor(()=>expect(chat.readConversation).toHaveBeenCalled());
 expect(screen.queryByRole("button",{name:/Start over/})).toBeNull();
 view.unmount();
 render(<ProjectPrompt projectId="project" runId="source-2" run={{...source,id:"source-2"}} archived onStartOver={onStartOver} onCreated={()=>{}}/>);
 const button=await screen.findByRole("button",{name:/Start over/});
 await waitFor(()=>expect(button.hasAttribute("disabled")).toBe(false));
 fireEvent.click(button);
 expect(onStartOver).toHaveBeenCalled();
 cleanup();
 const onOpenSource=vi.fn();
 render(<ProjectPrompt projectId="project" runId="fresh" run={{...source,id:"fresh",contextRunId:"source",contextTitle:"Cache design",contextOmitted:3}} onOpenSource={onOpenSource} onCreated={()=>{}}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Cache design"}));
 expect(onOpenSource).toHaveBeenCalledWith("source");
 screen.getByText(/3 earlier messages left out/);
});
