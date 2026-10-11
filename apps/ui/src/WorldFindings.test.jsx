import { render, screen, fireEvent } from "@testing-library/react";
import { describe, it, expect, vi } from "vitest";
import ProposalFindings from "./WorldFindings.jsx";
describe("Proposal consistency", () => {
  it("checks the inspected proposal base and labels semantic uncertainty", async () => {
    const api = vi.fn(async () => ({
      answer: "Possible conflict",
      deterministic: { passed: true, revision: 2, changeId: "proposal" },
      evidenceScope: { coverage: "selected_records" },
      findings: [
        {
          severity: "warning",
          uncertainty: "high",
          message: "Motive needs review",
          citations: ["world:2:main:hero"],
        },
      ],
    }));
    render(
      <ProposalFindings
        api={api}
        path="/world"
        change={{ id: "proposal", baseRevision: 2 }}
        timeline="main"
      />,
    );
    fireEvent.click(
      screen.getByRole("button", { name: "Check proposal consistency" }),
    );
    await screen.findByText("Motive needs review");
    expect(api.mock.calls[0][1]).toMatchObject({
      changeId: "proposal",
      revision: 2,
      workflow: "consistency_check",
    });
    expect(screen.getByText(/uncertainty: high/)).toBeTruthy();
    expect(screen.getByText(/selected evidence only/)).toBeTruthy();
    expect(
      screen
        .getByRole("link", { name: "world:2:main:hero" })
        .getAttribute("href"),
    ).toContain("revision=2");
  });
});
