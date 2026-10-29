import {afterEach,expect,test,vi} from "vitest";
import {cleanup,fireEvent,render,screen,waitFor} from "@testing-library/react";
import {ProviderPermissions} from "../src/components/ProviderPermissions";
import {saveProviderPermissions} from "../src/lib/workspace";
vi.mock("../src/lib/workspace",()=>({saveProviderPermissions:vi.fn()}));
afterEach(()=>{cleanup();vi.resetAllMocks();});
test("permission defaults change only through explicit saving",async()=>{
 vi.mocked(saveProviderPermissions).mockResolvedValue({mode:"plan"});const reload=vi.fn(async()=>{});
 render(<ProviderPermissions provider="claude" view={{provider:"claude",complete:false,steps:{},permissions:{mode:"acceptEdits"}}} onSaved={reload}/>);
 fireEvent.click(screen.getByText("Autorisations des conversations"));
 expect(saveProviderPermissions).not.toHaveBeenCalled();
 fireEvent.change(screen.getByLabelText("Mode Claude par défaut"),{target:{value:"plan"}});
 expect(saveProviderPermissions).not.toHaveBeenCalled();
 fireEvent.click(screen.getByRole("button",{name:"Enregistrer les autorisations Sectile"}));
 await waitFor(()=>expect(saveProviderPermissions).toHaveBeenCalledExactlyOnceWith({provider:"claude",mode:"plan"}));
 expect(await screen.findByText(/Préférence enregistrée/)).toBeTruthy();expect(reload).toHaveBeenCalledTimes(1);
});
