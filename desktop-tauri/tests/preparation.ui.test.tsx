import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {ProviderPreparation} from "../src/components/ProviderPreparation";
import * as api from "../src/lib/workspace";
vi.mock("../src/lib/workspace",()=>({providerReadiness:vi.fn(),installProviderSkills:vi.fn(),configureProviderMcp:vi.fn()}));
beforeEach(()=>{vi.resetAllMocks();vi.mocked(api.providerReadiness).mockResolvedValue({provider:"claude",complete:false,steps:{skills:{status:"pending"},mcp:{status:"pending",configuration:{present:true,valid:false}}}});});afterEach(cleanup);
test("MCP replacement is explicit and unrelated preparation does not execute or install on mount",async()=>{
 render(<ProviderPreparation provider="claude" projects={[{id:"project",name:"Project"}]}/>);
 fireEvent.click(screen.getByText("Skills et connexion MCP Sectile"));
 const consent=await screen.findByRole("checkbox");
 expect(screen.getByRole("button",{name:"Configurer le MCP local"}).hasAttribute("disabled")).toBe(true);
 expect(api.installProviderSkills).not.toHaveBeenCalled();expect(api.configureProviderMcp).not.toHaveBeenCalled();
 fireEvent.click(consent);fireEvent.click(screen.getByRole("button",{name:"Configurer le MCP local"}));
 await waitFor(()=>expect(api.configureProviderMcp).toHaveBeenCalledWith("claude",true));
});
test("installing skills requires an explicit source and structured success",async()=>{
 vi.mocked(api.installProviderSkills).mockResolvedValue({success:false,skills:{status:"failed"}});
 render(<ProviderPreparation provider="claude" projects={[{id:"source",name:"Source"}]}/>);fireEvent.click(screen.getByText("Skills et connexion MCP Sectile"));
 expect(screen.getByRole("button",{name:"Installer les skills"}).hasAttribute("disabled")).toBe(true);
 fireEvent.change(screen.getByLabelText("Projet source des skills claude"),{target:{value:"source"}});
 fireEvent.click(screen.getByRole("button",{name:"Installer les skills"}));
 expect(await screen.findByRole("alert")).toBeTruthy();
 expect(api.installProviderSkills).toHaveBeenCalledWith("claude","source");expect(api.configureProviderMcp).not.toHaveBeenCalled();
});

test("isolated companion MCP never offers replacement of shared provider registrations",async()=>{
 vi.mocked(api.providerReadiness).mockResolvedValue({provider:"codex",complete:false,steps:{mcp:{status:"success",scope:"session",configuration:{present:true,valid:true}}}});
 render(<ProviderPreparation provider="codex" projects={[]}/>);
 fireEvent.click(screen.getByText("Skills et connexion MCP Sectile"));
 await screen.findByText(/Connexion MCP isolée active/);
 expect(screen.queryByText("Configurer le MCP local")).toBeNull();
 expect(screen.queryByRole("checkbox")).toBeNull();
 expect(api.configureProviderMcp).not.toHaveBeenCalled();
});
