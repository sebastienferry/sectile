import { afterEach, expect, test, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { QuestionForm } from "../src/components/QuestionForm";
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
 fireEvent.change(screen.getByLabelText("Free response: Which scope?"),{target:{value:"Only the API"}});
 expect((screen.getByRole("radio",{name:/Large/}) as HTMLInputElement).checked).toBe(false);
 view.rerender(<QuestionForm approval={{...approval}} disabled={false} onAnswer={answer} onSkip={skip} />);
 expect((screen.getByLabelText("Free response: Which scope?") as HTMLInputElement).value).toBe("Only the API");
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
