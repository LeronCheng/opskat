import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { JevClassificationDetails } from "@/components/audit/JevClassificationDetails";

describe("JevClassificationDetails", () => {
  it("shows all primary scores without inventing a secondary classification", () => {
    render(
      <JevClassificationDetails
        source={JSON.stringify({
          status: "OK",
          primary: {
            probabilities: {
              SAFE_READ: 0.981,
              SENSITIVE_READ: 0.009,
              SAFE_CHANGE: 0.005,
              DANGEROUS_CHANGE: 0.004,
              UNKNOWN: 0.001,
            },
          },
        })}
      />
    );
    expect(screen.getAllByRole("progressbar")).toHaveLength(5);
    expect(screen.getByRole("progressbar", { name: "commandType.SAFE_READ" })).toHaveAttribute("aria-valuenow", "98.1");
    expect(screen.queryByText("audit.secondaryClassification")).not.toBeInTheDocument();
  });

  it("shows a missing-service reason without fabricated zero scores", () => {
    render(
      <JevClassificationDetails source={JSON.stringify({ status: "UNCONFIGURED", reason: "Jev API key missing" })} />
    );
    expect(screen.getByText("audit.classificationNoProbabilities")).toBeInTheDocument();
    expect(screen.getByText(/Jev API key missing/)).toBeInTheDocument();
    expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
  });

  it.each(["{not JSON", JSON.stringify({ primary: { probabilities: { SAFE_READ: 1.4 } } })])(
    "reports invalid stored metadata without exposing JSON or replacing it with zeros",
    (source) => {
      render(<JevClassificationDetails source={source} />);
      expect(screen.getByRole("alert")).toHaveTextContent("audit.classificationInvalid");
      expect(screen.queryByRole("progressbar")).not.toBeInTheDocument();
      expect(screen.queryByText(source)).not.toBeInTheDocument();
    }
  );
});
