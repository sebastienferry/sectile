import { afterEach, beforeEach, expect, test, vi } from "vitest";
import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { OAuthConnections } from "../src/components/OAuthConnections";
import * as api from "../src/lib/local";
vi.mock("../src/lib/local",()=>({oauthApps:vi.fn(),saveOAuthApp:vi.fn(),beginOAuth:vi.fn(),oauthStatus:vi.fn(),cancelOAuth:vi.fn()}));
const configured={github:{configured:true,clientIdSet:true,secretSet:true,redirectUrl:"http://127.0.0.1:1234/auth/github/callback",expectedRedirectUrl:"http://127.0.0.1:1234/auth/github/callback"}};
beforeEach(()=>{vi.resetAllMocks();vi.mocked(api.oauthApps).mockResolvedValue(configured);vi.mocked(api.cancelOAuth).mockResolvedValue(undefined);});
afterEach(()=>cleanup());
test("only confirmed OAuth completion refreshes connected account metadata",async()=>{
 const done=vi.fn();vi.mocked(api.beginOAuth).mockResolvedValue("flow");vi.mocked(api.oauthStatus).mockResolvedValue({status:"connected"});
 render(<OAuthConnections onConnected={done}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Connect to GitHub"}));
 await screen.findByText("Account connected. Your credentials are saved on this workstation.");
 expect(done).toHaveBeenCalledOnce();expect(api.cancelOAuth).not.toHaveBeenCalled();
});
test("denied consent is not reported as connected",async()=>{
 const done=vi.fn();vi.mocked(api.beginOAuth).mockResolvedValue("flow");vi.mocked(api.oauthStatus).mockResolvedValue({status:"cancelled"});
 render(<OAuthConnections onConnected={done}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Connect to GitHub"}));
 await screen.findByText("Sign-in canceled.");expect(done).not.toHaveBeenCalled();
});
test("cancelling an in-flight OAuth check keeps the cancelled outcome and cancels once",async()=>{
 let finishStatus!:(value:{status:string})=>void;
 const done=vi.fn();
 vi.mocked(api.beginOAuth).mockResolvedValue("flow");
 vi.mocked(api.oauthStatus).mockImplementation(()=>new Promise(resolve=>{finishStatus=resolve;}));
 render(<OAuthConnections onConnected={done}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Connect to GitHub"}));
 await waitFor(()=>expect(api.oauthStatus).toHaveBeenCalledWith("flow"));
 fireEvent.click(screen.getByRole("button",{name:"Cancel sign-in"}));
 await screen.findByText("Sign-in canceled.");
 await act(async()=>finishStatus({status:"connected"}));
 expect(done).not.toHaveBeenCalled();
 expect(screen.queryByText("Account connected. Your credentials are saved on this workstation.")).toBeNull();
 expect(api.cancelOAuth).toHaveBeenCalledTimes(1);
});
test("a disconnected OAuth account remains visible with a reconnect action",async()=>{
 render(<OAuthConnections onConnected={vi.fn()} credentials={[{tracker:"github",kind:"oauth",account:"Fixture",disconnected:true}]}/>);
 expect(await screen.findByRole("alert")).toHaveProperty("textContent","GitHub connection interrupted for Fixture. Reconnect to access your tickets.");
 expect(screen.queryByText("Connected account: Fixture")).toBeNull();
 expect((screen.getByRole("button",{name:"Connect to GitHub"}) as HTMLButtonElement).disabled).toBe(false);
});
test("Fleet Jira guidance follows tracker administration rights",async()=>{
 const member=render(<OAuthConnections onConnected={vi.fn()} serverManaged canManageTrackers={false}/>);
 expect(screen.getByText(/For Jira, ask an administrator to add your site/)).toBeTruthy();
 expect(screen.queryByText(/For Jira, also add your site/)).toBeNull();
 member.rerender(<OAuthConnections onConnected={vi.fn()} serverManaged canManageTrackers/>);
 expect(screen.getByText(/For Jira, also add your site in the ticket sources below/)).toBeTruthy();
});
test("leaving the view cancels a browser flow even if initiation resolves later",async()=>{
 let resolve!:(id:string)=>void;vi.mocked(api.beginOAuth).mockImplementation(()=>new Promise(done=>{resolve=done;}));
 const view=render(<OAuthConnections onConnected={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Connect to GitHub"}));view.unmount();
 await act(async()=>resolve("flow"));expect(api.cancelOAuth).toHaveBeenCalledWith("flow");
});
test("missing OAuth app configuration blocks connection and clears a submitted secret",async()=>{
 vi.mocked(api.oauthApps).mockResolvedValue({github:{...configured.github,configured:false,clientIdSet:false,secretSet:false}});
 vi.mocked(api.saveOAuthApp).mockRejectedValue(new Error("Save refused"));
 render(<OAuthConnections onConnected={vi.fn()}/>);
 expect((await screen.findByRole("button",{name:"Connect to GitHub"}) as HTMLButtonElement).disabled).toBe(true);
 fireEvent.click(screen.getByText("Configure OAuth application"));
 fireEvent.change(screen.getByLabelText("GitHub client ID"),{target:{value:"fixture-client"}});
 const secret=screen.getByLabelText("GitHub client secret") as HTMLInputElement;
 fireEvent.change(secret,{target:{value:"fixture-secret"}});
 fireEvent.click(screen.getByRole("button",{name:"Save GitHub application"}));
 await screen.findByRole("alert");await waitFor(()=>expect(secret.value).toBe(""));
 expect(api.saveOAuthApp).toHaveBeenCalledWith("github","fixture-client","fixture-secret");
});

test("saved OAuth credentials stay empty and masked while blank updates retain them",async()=>{
 vi.mocked(api.saveOAuthApp).mockResolvedValue(undefined);
 render(<OAuthConnections onConnected={vi.fn()}/>);
 fireEvent.click(await screen.findByText("OAuth application configured · edit"));
 const client=screen.getByLabelText("GitHub client ID") as HTMLInputElement;
 const secret=screen.getByLabelText("GitHub client secret") as HTMLInputElement;
 expect(client.type).toBe("password");expect(client.value).toBe("");expect(client.required).toBe(false);
 expect(secret.type).toBe("password");expect(secret.value).toBe("");expect(secret.required).toBe(false);
 fireEvent.click(screen.getByRole("button",{name:"Save GitHub application"}));
 await waitFor(()=>expect(api.saveOAuthApp).toHaveBeenCalledWith("github","",""));
 await screen.findByText("OAuth application saved. You can now sign in.");
 expect(client.value).toBe("");expect(secret.value).toBe("");
});
