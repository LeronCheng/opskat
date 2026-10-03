import { describe, it, expect, vi, beforeEach } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { CommandExplanation } from "@/components/approval/CommandExplanation";
import { ExplainCommand, CancelCommandExplanation } from "../../wailsjs/go/ai/AI";
import { EventsOn } from "../../wailsjs/runtime/runtime";

type StreamEvent = { sequence: number; content: string; done?: boolean; error?: string };
let receive: (event: StreamEvent) => void;
let unsubscribe: ReturnType<typeof vi.fn>;

describe("CommandExplanation", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    unsubscribe = vi.fn();
    vi.mocked(EventsOn).mockImplementation(((_name: string, handler: typeof receive) => {
      receive = handler;
      return unsubscribe;
    }) as never);
    vi.mocked(ExplainCommand).mockResolvedValue();
    vi.mocked(CancelCommandExplanation).mockResolvedValue();
  });

  it("renders streamed text while the explanation is still generating", () => {
    render(<CommandExplanation item={{ type: "exec", command: "pwd" }} />);
    fireEvent.click(screen.getByTestId("explain-command"));
    expect(receive).toBeTypeOf("function");
    act(() => receive({ sequence: 1, content: "Print the " }));
    expect(screen.getByTestId("command-explanation-answer")).toHaveTextContent("Print the");
    expect(screen.getByTestId("explain-command")).toBeDisabled();
    act(() => receive({ sequence: 2, content: "Print the current directory. Summary: show the path." }));
    expect(screen.getByTestId("command-explanation-answer")).toHaveTextContent("Print the current directory");
    act(() => receive({ sequence: 3, content: "Print the current directory. Summary: show the path.", done: true }));
    expect(screen.getByTestId("explain-command")).toBeEnabled();
    expect(unsubscribe).toHaveBeenCalled();
  });

  it("retains partial text on stream failure and starts a fresh answer when retrying", async () => {
    render(<CommandExplanation item={{ type: "exec", command: "pwd" }} />);
    fireEvent.click(screen.getByTestId("explain-command"));
    act(() => receive({ sequence: 1, content: "Partial answer" }));
    act(() => receive({ sequence: 2, content: "Partial answer", done: true, error: "stream interrupted" }));
    expect(screen.getByRole("alert")).toHaveTextContent("stream interrupted");
    expect(screen.getByTestId("command-explanation-answer")).toHaveTextContent("Partial answer");
    fireEvent.click(screen.getByTestId("explain-command"));
    expect(screen.queryByTestId("command-explanation-answer")).not.toBeInTheDocument();
    act(() => receive({ sequence: 1, content: "Effect and summary", done: true }));
    expect(screen.getByTestId("command-explanation-answer")).toHaveTextContent("Effect and summary");
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
  });

  it("surfaces startup failures inline", async () => {
    vi.mocked(ExplainCommand).mockRejectedValueOnce(new Error("provider unavailable"));
    render(<CommandExplanation item={{ type: "exec", command: "pwd" }} />);
    fireEvent.click(screen.getByTestId("explain-command"));
    expect(await screen.findByRole("alert")).toHaveTextContent("provider unavailable");
    expect(screen.getByTestId("explain-command")).toBeEnabled();
    expect(unsubscribe).toHaveBeenCalled();
  });

  it("keeps the latest full text when IPC stream events arrive out of order", () => {
    render(<CommandExplanation item={{ type: "exec", command: "pwd" }} />);
    fireEvent.click(screen.getByTestId("explain-command"));
    const stream = receive as (event: StreamEvent) => void;
    act(() => stream({ sequence: 1, content: "Print " }));
    act(() => stream({ sequence: 3, content: "Print current directory." }));
    act(() => stream({ sequence: 2, content: "Print current" }));
    expect(screen.getByTestId("command-explanation-answer")).toHaveTextContent("Print current directory.");
    act(() => stream({ sequence: 5, content: "Print current directory. Summary: show the path.", done: true }));
    act(() => stream({ sequence: 4, content: "Print current directory. Summary:" }));
    expect(screen.getByTestId("command-explanation-answer")).toHaveTextContent(
      "Print current directory. Summary: show the path."
    );
  });

  it("cancels after a delayed startup acknowledgement and ignores late text for a different approval", async () => {
    let acknowledge!: () => void;
    vi.mocked(ExplainCommand).mockReturnValue(
      new Promise<void>((resolve) => {
        acknowledge = resolve;
      })
    );
    const view = render(<CommandExplanation key="first" item={{ type: "exec", command: "pwd" }} />);
    fireEvent.click(screen.getByTestId("explain-command"));
    const requestID = vi.mocked(ExplainCommand).mock.calls[0][0];
    view.rerender(<CommandExplanation key="second" item={{ type: "exec", command: "ls" }} />);
    expect(unsubscribe).toHaveBeenCalled();
    act(() => receive({ sequence: 1, content: "Old answer" }));
    await act(async () => acknowledge());
    expect(CancelCommandExplanation).toHaveBeenCalledWith(requestID);
    expect(screen.queryByText("Old answer")).not.toBeInTheDocument();
    expect(screen.getByTestId("explain-command")).toBeEnabled();
  });
});
