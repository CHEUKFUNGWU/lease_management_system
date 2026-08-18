import { describe, it, expect, vi } from "vitest";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { CommandPalette } from "./CommandPalette";
import { AuthProvider } from "../../context/AuthContext";
import { LanguageProvider } from "../../context/LanguageContext";

vi.mock("next/navigation", () => ({
  useRouter: () => ({ push: vi.fn() }),
}));

describe("CommandPalette Deep Module", () => {
  it("renders closed palette without errors", () => {
    const html = renderToStaticMarkup(
      React.createElement(
        AuthProvider,
        null,
        React.createElement(
          LanguageProvider,
          null,
          React.createElement(CommandPalette, null)
        )
      )
    );

    // Initial closed state returns empty modal container
    expect(html).toBeDefined();
  });
});
