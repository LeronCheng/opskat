import { beforeEach, describe, expect, it, vi } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import { JevSettingsSection } from "@/components/settings/JevSettingsSection";
import {
  GetJevAPIKey,
  GetJevPrimaryConfidenceThreshold,
  SaveJevAPIKey,
  SaveJevPrimaryConfidenceThreshold,
} from "../../wailsjs/go/system/System";

describe("Jev settings", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(GetJevAPIKey).mockResolvedValue("existing-key");
    vi.mocked(GetJevPrimaryConfidenceThreshold).mockResolvedValue(0.5);
  });
  it("loads the hidden key and saves replacement or removal", async () => {
    render(<JevSettingsSection />);
    const input = screen.getByTestId("jev-api-key");
    await waitFor(() => expect(input).toHaveValue("existing-key"));
    expect(input).toHaveAttribute("type", "password");
    expect(screen.getByTestId("jev-primary-confidence")).toHaveValue("50");
    fireEvent.change(screen.getByTestId("jev-primary-confidence"), { target: { value: "65" } });
    fireEvent.change(input, { target: { value: "new-key" } });
    fireEvent.click(screen.getByTestId("jev-save"));
    await waitFor(() => expect(SaveJevAPIKey).toHaveBeenCalledWith("new-key"));
    await waitFor(() => expect(SaveJevPrimaryConfidenceThreshold).toHaveBeenCalledWith(0.65));
    await waitFor(() => expect(screen.getByTestId("jev-save")).not.toBeDisabled());
    fireEvent.change(input, { target: { value: "" } });
    fireEvent.click(screen.getByTestId("jev-save"));
    await waitFor(() => expect(SaveJevAPIKey).toHaveBeenCalledWith(""));
  });
  it("keeps the entered key when saving fails", async () => {
    vi.mocked(SaveJevAPIKey).mockRejectedValueOnce(new Error("disk unavailable"));
    render(<JevSettingsSection />);
    await waitFor(() => expect(screen.getByTestId("jev-api-key")).toHaveValue("existing-key"));
    fireEvent.click(screen.getByTestId("jev-save"));
    await waitFor(() => expect(screen.getByTestId("jev-save")).not.toBeDisabled());
    expect(screen.getByTestId("jev-api-key")).toHaveValue("existing-key");
  });
  it("allows replacement after the stored key cannot be decrypted", async () => {
    vi.mocked(GetJevAPIKey).mockRejectedValueOnce(new Error("key decryption failed"));
    render(<JevSettingsSection />);
    await waitFor(() => expect(screen.getByTestId("jev-api-key")).not.toBeDisabled());
    fireEvent.change(screen.getByTestId("jev-api-key"), { target: { value: "replacement-key" } });
    fireEvent.click(screen.getByTestId("jev-save"));
    await waitFor(() => expect(SaveJevAPIKey).toHaveBeenCalledWith("replacement-key"));
  });
});
