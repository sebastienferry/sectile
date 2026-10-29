import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { Markdown } from "../src/components/Markdown";
import { ConversationTranscript, transcriptRows } from "../src/components/ConversationTranscript";
import type { ConversationEvent, ConversationSnapshot } from "../src/lib/conversation";
afterEach(() => { cleanup(); vi.unstubAllGlobals(); });
const snapshot = (events: ConversationEvent[] = [], partial = ""): ConversationSnapshot => ({ id:"chat",busy:true,readOnly:false,events:events.map(event=>JSON.stringify(event)),partial });
test("Markdown renders headings, lists, tables and copyable fenced code without executing HTML or loading images", async () => {
 const writeText=vi.fn().mockResolvedValue(undefined);
 Object.defineProperty(navigator,"clipboard",{configurable:true,value:{writeText}});
 const {container}=render(<Markdown text={'# Title\n\n**Bold** and `inline`\n\n- One\n- Two\n\n| A | B |\n|---|---|\n| 1 | 2 |\n\n```ts\nconst answer = 42;\n```\n\n<img src="https://invalid.example/pixel" onerror="alert(1)">\n\n![remote](https://invalid.example/image)\n\n[unsafe](javascript:alert%281%29)'} />);
 expect(screen.getByRole("heading",{name:"Title"})).toBeTruthy();
 expect(container.querySelectorAll("li")).toHaveLength(2);
 expect(container.querySelector("table")).toBeTruthy();
 expect(container.querySelector("pre code")?.textContent).toBe("const answer = 42;");
 expect(container.querySelectorAll("img, script, iframe, a")).toHaveLength(0);
 fireEvent.click(screen.getByRole("button",{name:"Copier le code"}));
 await waitFor(()=>expect(writeText).toHaveBeenCalledWith("const answer = 42;"));
});
test("tool calls and out-of-order results remain paired by identity and collapse into a group", () => {
 const events:ConversationEvent[]=[{kind:"tool",tool:"Bash",toolId:"a",text:"Bash",detail:"pwd"},{kind:"tool",tool:"Read",toolId:"b",text:"Read"},{kind:"assistant",text:"Answer"},{kind:"tool_result",toolId:"b",text:"second result"},{kind:"tool_result",toolId:"a",text:"first result",error:true}];
 const rows=transcriptRows(events);
 expect(rows).toHaveLength(2);
 expect("tools" in rows[0] && rows[0].tools[0].result?.text).toBe("first result");
 const {container}=render(<ConversationTranscript snapshot={snapshot(events)} />);
 const group=container.querySelector("details")!;
 expect(group.open).toBe(false);
 expect(group.querySelector("summary")?.textContent).toContain("2 appels d’outils");
 expect(group.querySelector("summary")?.textContent).toContain("1 en erreur");
 expect(screen.getByText("Answer")).toBeTruthy();
});
test("an orphan tool result stays visible and does not attach to a different call", () => {
 const rows=transcriptRows([{kind:"tool",toolId:"a",text:"Bash"},{kind:"tool_result",toolId:"unknown",text:"orphan"}]);
 expect(rows).toHaveLength(2);
 expect("tools" in rows[0] && rows[0].tools[0].result).toBeUndefined();
});
test("following the response pauses on scroll-up and resumes only near the bottom or on request", () => {
 const view=render(<ConversationTranscript snapshot={snapshot([],"First")} />);
 const viewport=screen.getByLabelText("Historique de conversation");
 Object.defineProperties(viewport,{scrollHeight:{configurable:true,value:1000},clientHeight:{configurable:true,value:200}});
 view.rerender(<ConversationTranscript snapshot={snapshot([],"Second")} />);
 expect(viewport.scrollTop).toBe(1000);
 viewport.scrollTop=100;fireEvent.scroll(viewport);
 view.rerender(<ConversationTranscript snapshot={snapshot([],"Third")} />);
 expect(viewport.scrollTop).toBe(100);
 fireEvent.click(screen.getByRole("button",{name:/Aller en bas/}));
 expect(viewport.scrollTop).toBe(1000);
 expect(screen.queryByRole("button",{name:/Aller en bas/})).toBeNull();
 viewport.scrollTop=100;fireEvent.scroll(viewport);
 view.rerender(<ConversationTranscript snapshot={snapshot([],"Fourth")} followRequest={1} />);
 expect(viewport.scrollTop).toBe(1000);
});
test("partial fenced code remains readable before the closing fence arrives", () => {
 const view=render(<ConversationTranscript snapshot={snapshot([],"```js\nconsole.log(1)")} />);
 expect(view.container.querySelector("pre code")?.textContent).toBe("console.log(1)");
 view.rerender(<ConversationTranscript snapshot={snapshot([],"```js\nconsole.log(12)\n```\n\nDone")} />);
 expect(screen.getByText("Done")).toBeTruthy();
});

test("Markdown entities decode as text while inline code remains literal", () => {
 const {container}=render(<Markdown text={'A &amp; B &#x1F44B;\n\n`&amp;`'} />);
 expect(screen.getByText("A & B 👋")).toBeTruthy();
 expect(container.querySelector("code")?.textContent).toBe("&amp;");
});
test("content resize follows only while pinned, and observers disconnect on unmount", () => {
 let resize!: () => void;
 const disconnect=vi.fn();
 vi.stubGlobal("ResizeObserver",class {constructor(callback:()=>void){resize=callback;}observe(){}disconnect=disconnect;});
 const view=render(<ConversationTranscript snapshot={snapshot([],"Draft")} />);
 const viewport=screen.getByLabelText("Historique de conversation");
 Object.defineProperties(viewport,{scrollHeight:{configurable:true,value:1000},clientHeight:{configurable:true,value:200}});
 resize();expect(viewport.scrollTop).toBe(1000);
 viewport.scrollTop=100;fireEvent.scroll(viewport);
 resize();expect(viewport.scrollTop).toBe(100);
 viewport.scrollTop=800;fireEvent.scroll(viewport);
 Object.defineProperty(viewport,"scrollHeight",{configurable:true,value:1200});
 resize();expect(viewport.scrollTop).toBe(1200);
 view.unmount();expect(disconnect).toHaveBeenCalledTimes(1);
});

test("recovered streaming fragments are marked interrupted and keep their Markdown", () => {
 render(<ConversationTranscript snapshot={{...snapshot([{kind:"assistant",text:"Recovered **fragment**",interrupted:true}]),busy:false}} />);
 expect(screen.getByText("Assistant · réponse interrompue")).toBeTruthy();
 expect(screen.getByText("fragment").tagName).toBe("STRONG");
});
