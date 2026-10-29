import {afterEach,beforeEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {OrchestrationSettings} from "../src/components/OrchestrationSettings";
import {getOrchestration,runOrchestration} from "../src/lib/agent";
import {saveOrchestration} from "../src/lib/workspace";
vi.mock("../src/lib/agent",()=>({getOrchestration:vi.fn(),runOrchestration:vi.fn()}));vi.mock("../src/lib/workspace",()=>({saveOrchestration:vi.fn()}));
beforeEach(()=>{vi.resetAllMocks();vi.mocked(getOrchestration).mockResolvedValue({orchestratorEnabled:true,orchestratorMaxConcurrent:2,agents:[{id:"docs",name:"Docs",file:"docs.md",guidelines:"Review docs"}]});vi.mocked(saveOrchestration).mockResolvedValue({});});afterEach(cleanup);
test("saving an inactive policy does not execute it and dirty state prevents supervision navigation",async()=>{
 const supervise=vi.fn();render(<OrchestrationSettings projectId="project" onSupervise={supervise}/>);fireEvent.click(screen.getByText("Agents d’orchestration"));
 const enabled=await screen.findByRole("checkbox");fireEvent.click(enabled);
 expect(screen.getByRole("button",{name:"Ouvrir la supervision"}).hasAttribute("disabled")).toBe(true);
 fireEvent.click(screen.getByRole("button",{name:"Enregistrer l’orchestration"}));
 await waitFor(()=>expect(saveOrchestration).toHaveBeenCalledWith("project",{orchestratorEnabled:false,orchestratorMaxConcurrent:2,agents:[{id:"docs",name:"Docs",file:"docs.md"}]}));
 expect(runOrchestration).not.toHaveBeenCalled();expect(supervise).not.toHaveBeenCalled();
});
test("duplicate agent names cannot be saved",async()=>{
 render(<OrchestrationSettings projectId="project" onSupervise={()=>{}}/>);fireEvent.click(screen.getByText("Agents d’orchestration"));await screen.findByDisplayValue("Docs");
 fireEvent.click(screen.getByRole("button",{name:"Ajouter un agent"}));const names=screen.getAllByLabelText("Nom de l’agent");fireEvent.change(names[1],{target:{value:"docs"}});
 expect(screen.getByRole("button",{name:"Enregistrer l’orchestration"}).hasAttribute("disabled")).toBe(true);expect(saveOrchestration).not.toHaveBeenCalled();
});
