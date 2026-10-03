import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { AuditLogPage } from "@/components/audit/AuditLogPage";
import { ListAuditLogs, ListAuditSessions } from "../../wailsjs/go/system/System";

describe("AuditLogPage result status", () => {
  beforeEach(() => {
    vi.mocked(ListAuditSessions).mockResolvedValue([]);
  });

  it("renders a denied, unsuccessful audit row with the failure icon", async () => {
    vi.mocked(ListAuditLogs).mockResolvedValue({
      items: [
        {
          ID: 1,
          Source: "opsctl",
          ToolName: "cp",
          AssetID: 7,
          AssetName: "controlled-sftp",
          Command: "cp /tmp/payload.bin → controlled-sftp:/srv/payload.bin",
          Request: "{}",
          Result: "",
          Error: "operation denied: user denied",
          Success: 0,
          ConversationID: 0,
          GrantSessionID: "",
          SessionID: "opsctl-cp-deny",
          Decision: "deny",
          DecisionSource: "user_deny",
          MatchedPattern: "",
          Createtime: 1,
        },
      ],
      total: 1,
    } as never);

    render(<AuditLogPage />);

    expect(await screen.findByText("cp")).toBeInTheDocument();
    expect(screen.getByLabelText("audit.failed")).toBeInTheDocument();
    expect(screen.queryByLabelText("audit.success")).not.toBeInTheDocument();
  });

  it("lets users select audit data and detail payloads for keyboard copy", async () => {
    vi.mocked(ListAuditLogs).mockResolvedValue({
      items: [
        {
          ID: 2,
          Source: "opsctl",
          ToolName: "selectable-tool",
          AssetID: 8,
          AssetName: "selectable-asset",
          Command: "echo selectable-command",
          Request: '{"request":"selectable"}',
          Result: '{"response":"selectable"}',
          Error: "selectable-error",
          Success: 0,
          ConversationID: 0,
          GrantSessionID: "",
          SessionID: "selectable-session",
          Decision: "deny",
          DecisionSource: "policy_deny",
          MatchedPattern: "selectable-pattern",
          Createtime: 1,
        },
      ],
      total: 1,
    } as never);

    const user = userEvent.setup();
    render(<AuditLogPage />);

    const tool = await screen.findByText("selectable-tool");
    const row = tool.closest("tr");
    expect(row?.parentElement).toHaveClass("select-text");

    await user.click(within(row as HTMLTableRowElement).getByRole("button"));

    const request = screen.getByText('{"request":"selectable"}');
    const detail = request.closest('[role="dialog"]')?.querySelector(".select-text");
    expect(detail).toBeInTheDocument();
    expect(request).toHaveClass("select-text");
    expect(screen.getByText('{"response":"selectable"}')).toHaveClass("select-text");
    expect(screen.getByText("selectable-error")).toHaveClass("select-text");
  });

  it("shows primary and secondary probabilities as percentage bars instead of JSON", async () => {
    const classification = JSON.stringify({
      level1: "DANGEROUS_CHANGE",
      level2: ["CONFIG_CHANGE"],
      status: "OK",
      primary: {
        probabilities: {
          SAFE_READ: 0.01,
          SENSITIVE_READ: 0.02,
          SAFE_CHANGE: 0.03,
          DANGEROUS_CHANGE: 0.9,
          UNKNOWN: 0.04,
        },
      },
      secondary: { CONFIG_CHANGE: { noul: 0.91 }, SERVICE_HOST_CHANGE: { noul: 0.72 } },
    });
    vi.mocked(ListAuditLogs).mockResolvedValue({
      items: [
        {
          ID: 3,
          ToolName: "exec",
          Command: "systemctl restart nginx",
          CommandType: "DANGEROUS_CHANGE",
          Classification: classification,
          Createtime: 1,
        },
      ],
      total: 1,
    } as never);
    const user = userEvent.setup();
    render(<AuditLogPage />);
    const row = (await screen.findByText("exec")).closest("tr")!;
    await user.click(within(row).getByRole("button"));
    const details = within(screen.getByRole("dialog"));
    expect(details.getByRole("progressbar", { name: "commandType.DANGEROUS_CHANGE" })).toHaveAttribute(
      "aria-valuenow",
      "90"
    );
    expect(details.getByRole("progressbar", { name: "commandSubtype.CONFIG_CHANGE" })).toHaveAttribute(
      "aria-valuenow",
      "91"
    );
    expect(details.getByRole("progressbar", { name: "commandSubtype.SERVICE_HOST_CHANGE" })).toHaveAttribute(
      "aria-valuenow",
      "72"
    );
    expect(details.getAllByRole("progressbar")).toHaveLength(7);
    expect(details.queryByText(classification)).not.toBeInTheDocument();
  });
});
