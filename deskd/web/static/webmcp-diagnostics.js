"use strict";
// WebMCP read-only diagnostics — the service contract experiment.
//
// Only ever loaded when the server rendered the page with the dev flag on
// (web/web.go Deps.WebMCPDev, set from DESKD_WEBMCP_DEV=1 in cmd/deskd/main.go)
// — a deployed console never includes this file (Verify row 2, negative path).
//
// Hard boundaries (brief facts + Ground rules — do not relax without a new
// brief/review):
//   - dev-flag only: gated server-side, not here. This file assumes it was
//     only loaded because the flag is on.
//   - read-only by construction: every tool below only reads state already
//     present in the DOM/window. No tool issues a fetch with a method other
//     than GET (none issue a fetch at all), and no tool calls a state setter
//     (postForm/commitAnswer/ackItem/localStorage.setItem/etc). Enforced by
//     review AND by web/webmcp_test.go's static source-scan (Verify row 3).
//   - never a CI rung: WebMCP is a Chrome-only origin trial with no headless
//     support (talk 40). This file exists for a developer session driven via
//     a live Chrome DevTools MCP bridge — never for an automated check, and
//     the experiment result must never be cited to wire WebMCP into CI.
//   - one page only: included from inbox.html alone. Never add this include
//     to item.html or any view that offers the answer-write path.
//
// Five design rules this file follows (talk 40): one purpose per tool, no
// overlap; state-aware registration (the whole file is a no-op with no
// WebMCP support, or on any page other than the inbox); descriptions written
// for the model, not marketing copy; outputs are small — exactly the fields
// asked for, nothing more.

(function () {
  var mc = typeof navigator !== "undefined" ? navigator.modelContext : undefined;
  if (!mc || typeof mc.registerTool !== "function") {
    // Progressive enhancement: no WebMCP support in this browser. Register
    // nothing; change nothing else about the page (talk 40's fallback model).
    return;
  }

  function currentViewState() {
    var params = new URLSearchParams(window.location.search);
    var rows = document.querySelectorAll("#queue .row");
    var urgentEl = document.querySelector(".urgent-count");
    return {
      route: window.location.pathname,
      pane: document.documentElement.getAttribute("data-view") || "flat",
      filter: {
        cell: params.get("cell") || "",
        state: params.get("state") || "",
        kind: params.get("kind") || "",
        urgency: params.get("urgency") || "",
      },
      row_count: rows.length,
      urgent_count: urgentEl ? parseInt(urgentEl.getAttribute("data-urgent") || "0", 10) : 0,
    };
  }

  function textOf(el, selector) {
    var found = el.querySelector(selector);
    return found ? found.textContent.trim() : "";
  }

  function boardSnapshot() {
    return Array.prototype.map.call(document.querySelectorAll("#queue .row"), function (row) {
      return {
        id: row.getAttribute("data-id") || "",
        gold: row.getAttribute("data-gold") === "true",
        answered: row.getAttribute("data-answered") === "true",
        unparsed: row.getAttribute("data-unparsed") === "true",
        title: textOf(row, ".title"),
        kind: textOf(row, ".chip-kind"),
        urgency: textOf(row, ".chip-urg"),
      };
    });
  }

  mc.registerTool({
    name: "list_console_errors",
    description:
      "Return page console errors and unhandled promise rejections recorded " +
      "since this diagnostics script loaded on the deskd inbox. Read-only, " +
      "takes no arguments; returns an empty array when there are none.",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
    async execute() {
      var buf = (window.__webmcpDiagnostics && window.__webmcpDiagnostics.errors) || [];
      return { errors: buf.slice() };
    },
  });

  mc.registerTool({
    name: "describe_current_view",
    description:
      "Describe the deskd inbox view exactly as currently rendered: the " +
      "route, the active pane (flat/grouped), the applied URL filter " +
      "(cell/state/kind/urgency), and the row/urgent counts. Read-only, " +
      "takes no arguments.",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
    async execute() {
      return currentViewState();
    },
  });

  mc.registerTool({
    name: "get_board_snapshot",
    description:
      "Return the deskd inbox rows exactly as currently rendered on the " +
      "page (id, title, kind, urgency, and gold/answered/unparsed flags), " +
      "in on-screen order. Read-only, takes no arguments; returns only what " +
      "is already visible — nothing is fetched or re-derived.",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
    async execute() {
      return { rows: boardSnapshot() };
    },
  });
})();
