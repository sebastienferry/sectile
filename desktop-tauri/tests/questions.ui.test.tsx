import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QuestionForm, SKIPPED_ANSWER, recommendedLabel } from "../src/components/QuestionForm";
import type { Approval } from "../src/lib/conversation";
afterEach(cleanup);
const approval: Approval = { id:"question-1",tool:"AskUserQuestion",input:{questions:[
 {header:"Scope",question:"Which scope?",multiSelect:false,options:[{label:"Small",description:"One file"},{label:"Large",description:"All files"}]},
 {question:"Which checks?",multiSelect:true,options:[{label:"Unit"},{label:"Integration"}]},
]}};
test("Claude single and multiple choices require every answer and use the Go answer contract", async () => {
 const answer=vi.fn().mockResolvedValue(undefined);
 render(<QuestionForm approval={approval} disabled={false} onAnswer={answer} onSkip={vi.fn()} />);
 expect(screen.getByRole("button",{name:"Send answers"}).hasAttribute("disabled")).toBe(true);
 fireEvent.click(screen.getByRole("radio",{name:/Small/}));
 fireEvent.click(screen.getByRole("checkbox",{name:"Unit"}));
 fireEvent.click(screen.getByRole("checkbox",{name:"Integration"}));
 fireEvent.click(screen.getByRole("button",{name:"Send answers"}));
 await waitFor(()=>expect(answer).toHaveBeenCalledWith({"Which scope?":"Small","Which checks?":"Unit, Integration"}));
 expect(screen.getByRole("button",{name:"Answer sent"}).hasAttribute("disabled")).toBe(true);
});
test("free text replaces a single choice and drafts survive snapshot rerenders", async () => {
 const answer=vi.fn().mockResolvedValue(undefined); const skip=vi.fn();
 const view=render(<QuestionForm approval={approval} disabled={false} onAnswer={answer} onSkip={skip} />);
 fireEvent.click(screen.getByRole("radio",{name:/Large/}));
 fireEvent.click(screen.getByRole("tab",{name:/^Scope,/}));
 fireEvent.change(screen.getByLabelText("Free response: Which scope?"),{target:{value:"Only the API"}});
 expect((screen.getByRole("radio",{name:/Large/}) as HTMLInputElement).checked).toBe(false);
 view.rerender(<QuestionForm approval={{...approval}} disabled={false} onAnswer={answer} onSkip={skip} />);
 expect((screen.getByLabelText("Free response: Which scope?") as HTMLInputElement).value).toBe("Only the API");
 fireEvent.click(screen.getByRole("tab",{name:/^Q2,/}));
 fireEvent.change(screen.getByLabelText("Free response: Which checks?"),{target:{value:"Smoke tests"}});
 fireEvent.click(screen.getByRole("button",{name:"Send answers"}));
 await waitFor(()=>expect(answer).toHaveBeenCalledWith({"Which scope?":"Only the API","Which checks?":"Smoke tests"}));
});
test("Codex secret answers are masked, cleared on failure, and never echoed in the error", async () => {
 const secret={id:"codex-q",tool:"AskUserQuestion",input:{questions:[{question:"Secret?",isSecret:true}]}};
 const answer=vi.fn().mockRejectedValue(new Error("Echo secret-fixture"));
 render(<QuestionForm approval={secret} disabled={false} onAnswer={answer} onSkip={vi.fn()} />);
 const input=screen.getByLabelText("Free response: Secret?") as HTMLInputElement;
 expect(input.type).toBe("password");
 fireEvent.change(input,{target:{value:"secret-fixture"}});
 fireEvent.click(screen.getByRole("button",{name:"Send answers"}));
 await screen.findByRole("alert");
 expect(screen.getByRole("alert").textContent).not.toContain("secret-fixture");
 expect(input.value).toBe("");
});
test("submission stays pending and duplicate clicks do not answer twice", async () => {
 let resolve!:()=>void;
 const answer=vi.fn().mockReturnValue(new Promise<void>(done=>{resolve=done;}));
 render(<QuestionForm approval={{id:"free",tool:"AskUserQuestion",input:{questions:[{question:"Why?"}]}}} disabled={false} onAnswer={answer} onSkip={vi.fn()} />);
 fireEvent.change(screen.getByLabelText("Free response: Why?"),{target:{value:"Because"}});
 const submit=screen.getByRole("button",{name:"Send answers"});
 fireEvent.click(submit);fireEvent.click(submit);
 expect(answer).toHaveBeenCalledTimes(1);
 resolve();await screen.findByRole("button",{name:"Answer sent"});
});
test("unknown question formats can be skipped without granting tool permission", async () => {
 const skip=vi.fn().mockResolvedValue(undefined); const answer=vi.fn();
 render(<QuestionForm approval={{id:"invalid",tool:"AskUserQuestion",input:{questions:[]}}} disabled={false} onAnswer={answer} onSkip={skip} />);
 await screen.findByRole("alert");
 fireEvent.click(screen.getByRole("button",{name:"Skip request"}));
 await waitFor(()=>expect(skip).toHaveBeenCalledTimes(1));
 expect(answer).not.toHaveBeenCalled();
});
const three: Approval = { id:"question-3",tool:"AskUserQuestion",input:{questions:[
 {header:"Scope",question:"Which scope?",multiSelect:false,options:[{label:"Small"},{label:"Large (Recommended)",description:"All files"}]},
 {header:"Tests",question:"Which checks?",multiSelect:true,options:[{label:"Unit"},{label:"Integration"}]},
 {header:"",question:"Anything else?",multiSelect:false,options:[]},
]}};
const selectedTab=()=>screen.getAllByRole("tab").find(tab=>tab.getAttribute("aria-selected")==="true")?.getAttribute("aria-label");
test("recommended suffix detection is exact, trailing and case-sensitive", () => {
 expect(recommendedLabel("Use SQLite (Recommended)")).toEqual({text:"Use SQLite",recommended:true});
 expect(recommendedLabel("Use SQLite   (Recommended)  ")).toEqual({text:"Use SQLite",recommended:true});
 expect(recommendedLabel("Use SQLite")).toEqual({text:"Use SQLite",recommended:false});
 expect(recommendedLabel("Use (Recommended) SQLite")).toEqual({text:"Use (Recommended) SQLite",recommended:false});
 expect(recommendedLabel("Use SQLite (recommended)")).toEqual({text:"Use SQLite (recommended)",recommended:false});
 expect(recommendedLabel("(Recommended)").recommended).toBe(false);
});
test("one tab per question, labelled by header or position, first selected and all to answer", () => {
 render(<QuestionForm approval={three} disabled={false} onAnswer={vi.fn()} onSkip={vi.fn()} />);
 expect(screen.getAllByRole("tab").map(tab=>tab.getAttribute("aria-label"))).toEqual(["Scope, to answer","Tests, to answer","Q3, to answer"]);
 expect(selectedTab()).toBe("Scope, to answer");
 expect(screen.getByRole("tabpanel").textContent).toContain("Which scope?");
 expect(screen.queryByText("Which checks?")).toBeNull();
 expect(screen.getByRole("button",{name:"Send answers"}).hasAttribute("disabled")).toBe(true);
});
test("a single question shows no tab strip", () => {
 render(<QuestionForm approval={{id:"one",tool:"AskUserQuestion",input:{questions:[{question:"Why?",options:[{label:"Because"}]}]}}} disabled={false} onAnswer={vi.fn()} onSkip={vi.fn()} />);
 expect(screen.queryByRole("tablist")).toBeNull();
 expect(screen.getByRole("radio",{name:/Because/})).toBeTruthy();
 expect(screen.getByRole("button",{name:"Skip request"})).toBeTruthy();
 expect(screen.getByRole("button",{name:"Skip this question"})).toBeTruthy();
});
test("single choice advances, multiple choice and typing do not", () => {
 render(<QuestionForm approval={three} disabled={false} onAnswer={vi.fn()} onSkip={vi.fn()} />);
 fireEvent.click(screen.getByRole("radio",{name:/Small/}));
 expect(screen.getByRole("tab",{name:"Scope, answered"})).toBeTruthy();
 expect(selectedTab()).toBe("Tests, to answer");
 fireEvent.click(screen.getByRole("checkbox",{name:"Unit"}));
 fireEvent.click(screen.getByRole("checkbox",{name:"Integration"}));
 expect(selectedTab()).toBe("Tests, answered");
 fireEvent.change(screen.getByLabelText("Free response: Which checks?"),{target:{value:"S"}});
 expect(selectedTab()).toBe("Tests, answered");
 fireEvent.click(screen.getByRole("tab",{name:/^Q3,/}));
 fireEvent.change(screen.getByLabelText("Free response: Anything else?"),{target:{value:"N"}});
 expect(selectedTab()).toBe("Q3, answered");
});
test("browsing single-choice options with the arrow keys answers without leaving the tab", () => {
 render(<QuestionForm approval={three} disabled={false} onAnswer={vi.fn()} onSkip={vi.fn()} />);
 const small=screen.getByRole("radio",{name:/Small/});
 fireEvent.keyDown(small,{key:"ArrowDown"});
 fireEvent.click(screen.getByRole("radio",{name:/Large/}));
 expect(selectedTab()).toBe("Scope, answered");
 fireEvent.keyDown(screen.getByRole("radio",{name:/Large/}),{key:" "});
 fireEvent.click(small);
 expect(selectedTab()).toBe("Tests, to answer");
});
test("a recommended option is highlighted, not preselected, and sent with its original label", async () => {
 const answer=vi.fn().mockResolvedValue(undefined);
 render(<QuestionForm approval={three} disabled={false} onAnswer={answer} onSkip={vi.fn()} />);
 const radio=screen.getByRole("radio",{name:/Large/}) as HTMLInputElement;
 expect(radio.checked).toBe(false);
 const option=radio.closest("label")!;
 expect(option.className).toContain("recommended");
 expect(option.textContent).not.toContain("(Recommended)");
 expect(option.querySelector(".question-badge")?.textContent).toBe("Recommended");
 expect((screen.getByRole("radio",{name:/Small/}).closest("label")!).className).not.toContain("recommended");
 fireEvent.click(radio);
 fireEvent.click(screen.getByRole("checkbox",{name:"Unit"}));
 fireEvent.click(screen.getByRole("tab",{name:/^Q3,/}));
 fireEvent.change(screen.getByLabelText("Free response: Anything else?"),{target:{value:"No"}});
 fireEvent.click(screen.getByRole("button",{name:"Send answers"}));
 await waitFor(()=>expect(answer).toHaveBeenCalledWith({"Which scope?":"Large (Recommended)","Which checks?":"Unit","Anything else?":"No"}));
});
test("a skipped question is sent as the skip literal and the others as chosen", async () => {
 const answer=vi.fn().mockResolvedValue(undefined);
 render(<QuestionForm approval={three} disabled={false} onAnswer={answer} onSkip={vi.fn()} />);
 fireEvent.click(screen.getByRole("radio",{name:/Small/}));
 fireEvent.click(screen.getByRole("checkbox",{name:"Unit"}));
 fireEvent.click(screen.getByRole("button",{name:"Skip this question"}));
 expect(screen.getByRole("tab",{name:"Tests, skipped"})).toBeTruthy();
 expect(selectedTab()).toBe("Q3, to answer");
 fireEvent.change(screen.getByLabelText("Free response: Anything else?"),{target:{value:"No"}});
 fireEvent.click(screen.getByRole("button",{name:"Send answers"}));
 await waitFor(()=>expect(answer).toHaveBeenCalledWith({"Which scope?":"Small","Which checks?":SKIPPED_ANSWER,"Anything else?":"No"}));
 expect(SKIPPED_ANSWER).toBe("Skipped by the owner");
});
test("every question skipped answers the request instead of denying it", async () => {
 const answer=vi.fn().mockResolvedValue(undefined); const skip=vi.fn();
 render(<QuestionForm approval={three} disabled={false} onAnswer={answer} onSkip={skip} />);
 for (let i=0;i<3;i++) fireEvent.click(screen.getByRole("button",{name:"Skip this question"}));
 fireEvent.click(screen.getByRole("button",{name:"Send answers"}));
 await waitFor(()=>expect(answer).toHaveBeenCalledWith({"Which scope?":SKIPPED_ANSWER,"Which checks?":SKIPPED_ANSWER,"Anything else?":SKIPPED_ANSWER}));
 expect(skip).not.toHaveBeenCalled();
});
test("answering a skipped question makes it answered again", () => {
 render(<QuestionForm approval={three} disabled={false} onAnswer={vi.fn()} onSkip={vi.fn()} />);
 fireEvent.click(screen.getByRole("tab",{name:/^Tests,/}));
 fireEvent.click(screen.getByRole("button",{name:"Skip this question"}));
 fireEvent.click(screen.getByRole("tab",{name:"Tests, skipped"}));
 fireEvent.click(screen.getByRole("checkbox",{name:"Unit"}));
 expect(screen.getByRole("tab",{name:"Tests, answered"})).toBeTruthy();
 expect(screen.getByRole("button",{name:"Skip this question"})).toBeTruthy();
});
test("skip request declines the whole request whatever the tabs' states", async () => {
 const answer=vi.fn(); const skip=vi.fn().mockResolvedValue(undefined);
 render(<QuestionForm approval={three} disabled={false} onAnswer={answer} onSkip={skip} />);
 fireEvent.click(screen.getByRole("radio",{name:/Small/}));
 fireEvent.click(screen.getByRole("button",{name:"Skip request"}));
 await waitFor(()=>expect(skip).toHaveBeenCalledTimes(1));
 expect(answer).not.toHaveBeenCalled();
});
test("answers over 16 KB disable sending and show the limit", () => {
 render(<QuestionForm approval={{id:"big",tool:"AskUserQuestion",input:{questions:[{question:"A?"},{question:"B?"}]}}} disabled={false} onAnswer={vi.fn()} onSkip={vi.fn()} />);
 fireEvent.change(screen.getByLabelText("Free response: A?"),{target:{value:"x".repeat(9000)}});
 fireEvent.click(screen.getByRole("tab",{name:/^Q2,/}));
 fireEvent.change(screen.getByLabelText("Free response: B?"),{target:{value:"y".repeat(9000)}});
 expect(screen.getByRole("alert").textContent).toContain("16 KB");
 expect(screen.getByRole("button",{name:"Send answers"}).hasAttribute("disabled")).toBe(true);
});
test("arrow keys move between tabs and keep entered values", () => {
 render(<QuestionForm approval={three} disabled={false} onAnswer={vi.fn()} onSkip={vi.fn()} />);
 fireEvent.change(screen.getByLabelText("Free response: Which scope?"),{target:{value:"Mine"}});
 const first=screen.getByRole("tab",{name:/^Scope,/});
 expect(first.tabIndex).toBe(0);
 fireEvent.keyDown(first,{key:"ArrowRight"});
 expect(selectedTab()).toBe("Tests, to answer");
 expect(document.activeElement).toBe(screen.getByRole("tab",{name:/^Tests,/}));
 fireEvent.keyDown(document.activeElement!,{key:"End"});
 expect(selectedTab()).toBe("Q3, to answer");
 fireEvent.keyDown(document.activeElement!,{key:"ArrowRight"});
 expect(selectedTab()).toBe("Scope, answered");
 expect((screen.getByLabelText("Free response: Which scope?") as HTMLInputElement).value).toBe("Mine");
 fireEvent.keyDown(document.activeElement!,{key:"ArrowLeft"});
 expect(selectedTab()).toBe("Q3, to answer");
});
