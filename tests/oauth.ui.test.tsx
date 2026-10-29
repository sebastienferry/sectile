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
 fireEvent.click(await screen.findByRole("button",{name:"Se connecter à GitHub"}));
 await screen.findByText("Compte connecté. Vos accès sont enregistrés sur ce poste.");
 expect(done).toHaveBeenCalledOnce();expect(api.cancelOAuth).not.toHaveBeenCalled();
});
test("denied consent is not reported as connected",async()=>{
 const done=vi.fn();vi.mocked(api.beginOAuth).mockResolvedValue("flow");vi.mocked(api.oauthStatus).mockResolvedValue({status:"cancelled"});
 render(<OAuthConnections onConnected={done}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Se connecter à GitHub"}));
 await screen.findByText("Connexion annulée.");expect(done).not.toHaveBeenCalled();
});
test("a disconnected OAuth account remains visible with a reconnect action",async()=>{
 render(<OAuthConnections onConnected={vi.fn()} credentials={[{tracker:"github",kind:"oauth",account:"Fixture",disconnected:true}]}/>);
 expect(await screen.findByRole("alert")).toHaveProperty("textContent","Connexion GitHub interrompue pour Fixture. Reconnectez-vous pour accéder à vos tickets.");
 expect(screen.queryByText("Compte connecté : Fixture")).toBeNull();
 expect((screen.getByRole("button",{name:"Se connecter à GitHub"}) as HTMLButtonElement).disabled).toBe(false);
});
test("Fleet Jira guidance follows tracker administration rights",async()=>{
 const member=render(<OAuthConnections onConnected={vi.fn()} serverManaged canManageTrackers={false}/>);
 expect(screen.getByText(/Pour Jira, demandez à un administrateur d’ajouter votre site/)).toBeTruthy();
 expect(screen.queryByText(/Pour Jira, ajoutez aussi votre site/)).toBeNull();
 member.rerender(<OAuthConnections onConnected={vi.fn()} serverManaged canManageTrackers/>);
 expect(screen.getByText(/Pour Jira, ajoutez aussi votre site dans les sources de tickets ci-dessous/)).toBeTruthy();
});
test("leaving the view cancels a browser flow even if initiation resolves later",async()=>{
 let resolve!:(id:string)=>void;vi.mocked(api.beginOAuth).mockImplementation(()=>new Promise(done=>{resolve=done;}));
 const view=render(<OAuthConnections onConnected={vi.fn()}/>);
 fireEvent.click(await screen.findByRole("button",{name:"Se connecter à GitHub"}));view.unmount();
 await act(async()=>resolve("flow"));expect(api.cancelOAuth).toHaveBeenCalledWith("flow");
});
test("missing OAuth app configuration blocks connection and clears a submitted secret",async()=>{
 vi.mocked(api.oauthApps).mockResolvedValue({github:{...configured.github,configured:false,clientIdSet:false,secretSet:false}});
 vi.mocked(api.saveOAuthApp).mockRejectedValue(new Error("Save refused"));
 render(<OAuthConnections onConnected={vi.fn()}/>);
 expect((await screen.findByRole("button",{name:"Se connecter à GitHub"}) as HTMLButtonElement).disabled).toBe(true);
 fireEvent.click(screen.getByText("Configurer l’application OAuth"));
 fireEvent.change(screen.getByLabelText("Identifiant client GitHub"),{target:{value:"fixture-client"}});
 const secret=screen.getByLabelText("Secret client GitHub") as HTMLInputElement;
 fireEvent.change(secret,{target:{value:"fixture-secret"}});
 fireEvent.click(screen.getByRole("button",{name:"Enregistrer l’application GitHub"}));
 await screen.findByRole("alert");await waitFor(()=>expect(secret.value).toBe(""));
 expect(api.saveOAuthApp).toHaveBeenCalledWith("github","fixture-client","fixture-secret");
});

test("saved OAuth credentials stay empty and masked while blank updates retain them",async()=>{
 vi.mocked(api.saveOAuthApp).mockResolvedValue(undefined);
 render(<OAuthConnections onConnected={vi.fn()}/>);
 fireEvent.click(await screen.findByText("Application OAuth configurée · modifier"));
 const client=screen.getByLabelText("Identifiant client GitHub") as HTMLInputElement;
 const secret=screen.getByLabelText("Secret client GitHub") as HTMLInputElement;
 expect(client.type).toBe("password");expect(client.value).toBe("");expect(client.required).toBe(false);
 expect(secret.type).toBe("password");expect(secret.value).toBe("");expect(secret.required).toBe(false);
 fireEvent.click(screen.getByRole("button",{name:"Enregistrer l’application GitHub"}));
 await waitFor(()=>expect(api.saveOAuthApp).toHaveBeenCalledWith("github","",""));
 await screen.findByText("Application OAuth enregistrée. Vous pouvez vous connecter.");
 expect(client.value).toBe("");expect(secret.value).toBe("");
});
