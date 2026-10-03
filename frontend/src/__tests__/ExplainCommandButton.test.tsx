import { beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { ApprovalBlock } from "@/components/approval/ApprovalBlock";
import { useAIStore, type ContentBlock } from "@/stores/aiStore";
import { useTabStore } from "@/stores/tabStore";
import { ExplainCommand, CreateConversation, SendAIMessage, RespondAIApproval } from "../../wailsjs/go/ai/AI";
import { EventsOn } from "../../wailsjs/runtime/runtime";

let receive: (event: { sequence: number; content: string; done?: boolean }) => void;

describe("Explain command", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    useTabStore.setState({
      tabs: [
        { id: "original", type: "ai", label: "Original", meta: { type: "ai", conversationId: 41, title: "Original" } },
      ],
      activeTabId: "original",
    });
    useAIStore.setState({
      tabStates: { original: {} },
      conversations: [],
      conversationMessages: { 41: [{ role: "user", content: "original private context", blocks: [] }] },
      conversationStreaming: { 41: { sending: true, pendingQueue: [] } },
      sidebarTabs: [],
      modelName: "test-model",
    });
    vi.mocked(ExplainCommand).mockResolvedValue();
    vi.mocked(EventsOn).mockImplementation(((_name: string, handler: typeof receive) => {
      receive = handler;
      return vi.fn();
    }) as never);
  });

  it("shows only the answer beneath the command while the original conversation and approval remain untouched", async () => {
    const block = {
      type: "approval",
      status: "pending_confirm",
      confirmId: "original-confirm",
      approvalKind: "single",
      approvalItems: [{ type: "exec", asset_id: 1, asset_name: "web-01", command: "systemctl restart nginx" }],
    } as ContentBlock;
    render(<ApprovalBlock block={block} />);
    fireEvent.click(screen.getByTestId("explain-command"));
    act(() => receive({ sequence: 1, content: "重启 nginx 服务。\n\n**总结**：使服务重新启动。", done: true }));
    await waitFor(() => expect(screen.getByTestId("command-explanation-answer")).toHaveTextContent("重启 nginx 服务"));
    expect(ExplainCommand).toHaveBeenCalledWith(expect.any(String), {
      type: "exec",
      command: "systemctl restart nginx",
      asset_name: "web-01",
      detail: "",
    });
    expect(screen.getByTestId("command-explanation-answer")).toHaveTextContent("总结");
    expect(CreateConversation).not.toHaveBeenCalled();
    expect(SendAIMessage).not.toHaveBeenCalled();
    expect(useAIStore.getState().conversationMessages[41][0].content).toBe("original private context");
    expect(useAIStore.getState().conversationStreaming[41].sending).toBe(true);
    expect(RespondAIApproval).not.toHaveBeenCalled();
    expect(screen.getByTestId("ai-approval-allow")).toBeInTheDocument();
    expect(useTabStore.getState().tabs).toHaveLength(1);
    expect(useTabStore.getState().activeTabId).toBe("original");
    expect(screen.queryByText("original private context")).not.toBeInTheDocument();
  });
});
