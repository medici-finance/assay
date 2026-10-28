"use strict";
// deskd console client. Two hard rules this file lives by:
//   1. Digit answer-bindings come from the option ELEMENTS the server rendered
//      from the typed enum — never from body text (§6.6). We read
//      data-option-id / data-digit off <li class="option">, and nothing else.
//   2. Two-step commit (D1): a digit/click STAGES; a DISTINCT key (Enter) or the
//      commit button COMMITS. Stage key !== commit key, so keyboard auto-repeat
//      can never stage-and-commit. gold rows never stage in-list at all.

// Display only: this server-rendered value never grants request authority.
function humanAuthor() {
  return document.body.dataset.humanLogin || "the configured human";
}

function announce(msg) {
  const live = document.getElementById("live");
  if (live) live.textContent = msg;
}

function refFromURL(url) {
  // https://github.com/<owner>/<name>/issues/<n> -> owner/name#n
  const m = /github\.com\/([^/]+)\/([^/]+)\/issues\/(\d+)/.exec(url || "");
  return m ? `${m[1]}/${m[2]}#${m[3]}` : "";
}

async function postForm(url, data) {
  const body = new URLSearchParams(data || {});
  const resp = await fetch(url, {
    method: "POST",
    credentials: "same-origin",
    headers: { "Content-Type": "application/x-www-form-urlencoded" },
    body,
  });
  let payload = {};
  try { payload = await resp.json(); } catch (e) { /* non-JSON */ }
  return { ok: resp.ok, status: resp.status, payload };
}

// ---- session bootstrap -----------------------------------------------------

async function refreshSession() {
  const chip = document.getElementById("session-chip");
  if (!chip) return { live: false, present: false };
  try {
    const resp = await fetch("/session", { credentials: "same-origin" });
    const s = await resp.json();
    chip.dataset.live = String(!!s.live);
    if (s.live) {
      chip.textContent = `session: live (${s.actor})`;
    } else if (s.credential_present) {
      chip.textContent = "session: start (click)";
      chip.onclick = beginSession;
      chip.style.cursor = "pointer";
    } else {
      chip.textContent = "human actions not authorized";
    }
    return s;
  } catch (e) {
    chip.textContent = "session: unknown";
    return { live: false, present: false };
  }
}

async function beginSession() {
  const r = await postForm("/session/begin", {});
  if (r.ok) {
    announce(`live session started, answering as ${humanAuthor()}`);
  } else {
    announce("could not start session: " + (r.payload.error || r.status));
  }
  await refreshSession();
}

// ---- staging + commit (shared) ---------------------------------------------

// State holds the single staged option for the current context.
const staged = { el: null, id: "", digit: 0 };

function clearStage() {
  if (staged.el) staged.el.classList.remove("staged");
  staged.el = null; staged.id = ""; staged.digit = 0;
  const chip = document.getElementById("confirm-chip");
  if (chip) chip.hidden = true;
  const commit = document.getElementById("commit");
  if (commit) commit.disabled = true;
}

function stageOption(el, ctx) {
  if (!el) return;
  clearStage();
  staged.el = el;
  staged.id = el.dataset.optionId;
  staged.digit = parseInt(el.dataset.digit || "0", 10);
  el.classList.add("staged");
  const target = refFromURL(ctx.platformURL);
  const label = (el.querySelector(".opt-label") || {}).textContent || "";
  const conseq = (el.querySelector(".opt-consequence") || {}).textContent || "";

  // confirm chip (item view)
  const chip = document.getElementById("confirm-chip");
  if (chip) {
    chip.hidden = false;
    const t = document.getElementById("confirm-target");
    const o = document.getElementById("confirm-option");
    if (t) t.textContent = target;
    if (o) o.textContent = `${staged.digit}: ${label}`;
  }
  const commit = document.getElementById("commit");
  if (commit) commit.disabled = false;

  announce(`Staged option ${staged.digit}: ${label}. ${conseq}. Enter to answer as ${humanAuthor()} and close ${target}; Escape to cancel`);
}

async function commitAnswer(ctx) {
  if (!staged.id && !(ctx.textEl && ctx.textEl.value.trim())) {
    announce("nothing staged to answer");
    return;
  }
  const status = document.getElementById("commit-status");
  const r = await postForm(`/answer/${ctx.id}`, {
    option: staged.id,
    options_rev: ctx.optionsRev || "",
    text: ctx.textEl ? ctx.textEl.value : "",
  });
  if (r.ok) {
    if (status) { status.textContent = `answered as ${humanAuthor()}, closed`; status.className = "commit-status ok"; }
    announce(`answered as ${humanAuthor()} and closed ` + refFromURL(ctx.platformURL));
    clearStage();
    setTimeout(() => { window.location.href = "/inbox"; }, 700);
  } else {
    const msg = (r.payload && r.payload.error) || `error ${r.status}`;
    if (status) { status.textContent = msg; status.className = "commit-status err"; }
    announce("answer not committed: " + msg + " — draft kept");
  }
}

async function ackItem(ctx) {
  const r = await postForm(`/answer/${ctx.id}/ack`, {});
  if (r.ok) announce(`acked as ${humanAuthor()} (👀); escalation stays open`);
  else announce("ack failed: " + ((r.payload && r.payload.error) || r.status));
}

// ---- inbox keyboard / peek -------------------------------------------------

function initInbox() {
  const queue = document.getElementById("queue");
  if (!queue) return;
  const rows = Array.from(document.querySelectorAll(".row"));
  if (rows.length === 0) return;
  let sel = 0;

  function select(i) {
    rows.forEach((r) => r.classList.remove("selected"));
    sel = Math.max(0, Math.min(rows.length - 1, i));
    rows[sel].classList.add("selected");
    rows[sel].scrollIntoView({ block: "nearest" });
    // peek travels with selection: close others' peeks (spec §2.1)
    rows.forEach((r) => { const p = r.querySelector(".peek"); if (p) p.hidden = true; });
    clearStage();
  }
  select(0);

  function ctxOf(row) {
    return {
      id: row.dataset.id,
      gold: row.dataset.gold === "true",
      optionsRev: row.dataset.optionsRev,
      platformURL: row.dataset.platformUrl,
      unparsed: row.dataset.unparsed === "true",
    };
  }

  document.addEventListener("keydown", (e) => {
    const row = rows[sel];
    const peek = row.querySelector(".peek");
    const ctx = ctxOf(row);
    switch (e.key) {
      case "j": select(sel + 1); e.preventDefault(); break;
      case "k": select(sel - 1); e.preventDefault(); break;
      case " ": // Space toggles the in-place peek
        if (peek) { peek.hidden = !peek.hidden; }
        e.preventDefault();
        break;
      case "a":
        ackItem(ctx); e.preventDefault(); break;
      case "Escape":
        clearStage(); e.preventDefault(); break;
      case "Enter":
        if (staged.id) { commitAnswer(ctx); }       // commit the staged option
        else { window.location.href = `/item/${ctx.id}`; } // else open the item
        e.preventDefault();
        break;
      default:
        if (/^[1-9]$/.test(e.key)) {
          // Digit STAGES — and only when the peek is open, the row is not gold,
          // and the row is not a malformed/unparsed escalation (§6.6, D1).
          if (peek && !peek.hidden && !ctx.gold && !ctx.unparsed) {
            const opt = peek.querySelector(`.option[data-digit="${e.key}"]`);
            if (opt) { stageOption(opt, ctx); e.preventDefault(); }
          } else if (ctx.gold) {
            announce("gold / irreversible — open the full item view to answer");
          }
        }
    }
  });
}

// ---- item detail -----------------------------------------------------------

function initItem() {
  const detail = document.querySelector("main.detail");
  if (!detail) return;
  const ctx = {
    id: detail.dataset.id,
    gold: detail.dataset.gold === "true",
    optionsRev: detail.dataset.optionsRev,
    platformURL: detail.dataset.platformUrl,
    textEl: document.getElementById("answer-text"),
  };

  // stage on click / keyboard-activate of an option
  document.querySelectorAll("#options .option").forEach((el) => {
    el.addEventListener("click", () => stageOption(el, ctx));
    el.addEventListener("keydown", (e) => {
      if (e.key === "Enter" || e.key === " ") { stageOption(el, ctx); e.preventDefault(); }
    });
  });

  // digit stages here too (full item view); Enter commits
  document.addEventListener("keydown", (e) => {
    if (e.target === ctx.textEl) return; // don't hijack typing in the note
    if (/^[1-9]$/.test(e.key)) {
      const opt = document.querySelector(`#options .option[data-digit="${e.key}"]`);
      if (opt) { stageOption(opt, ctx); e.preventDefault(); }
    } else if (e.key === "Enter" && staged.id) {
      commitAnswer(ctx); e.preventDefault();
    } else if (e.key === "Escape") {
      clearStage(); e.preventDefault();
    }
  });

  const commit = document.getElementById("commit");
  if (commit) commit.addEventListener("click", () => commitAnswer(ctx));
  const ack = document.getElementById("ack");
  if (ack) ack.addEventListener("click", () => ackItem(ctx));

  // draft persistence: load any saved draft, save on edit (debounced)
  loadDraft(ctx);
  if (ctx.textEl) {
    let t = null;
    ctx.textEl.addEventListener("input", () => {
      clearTimeout(t);
      t = setTimeout(() => saveDraft(ctx), 500);
    });
  }
}

async function loadDraft(ctx) {
  try {
    const resp = await fetch(`/answer/${ctx.id}/draft`, { credentials: "same-origin" });
    if (!resp.ok) return;
    const d = await resp.json();
    if (d.exists && ctx.textEl) {
      ctx.textEl.value = d.draft.text || "";
      announce("restored a saved draft");
    }
  } catch (e) { /* no draft */ }
}

async function saveDraft(ctx) {
  await postForm(`/answer/${ctx.id}/draft`, {
    option: staged.id || "",
    text: ctx.textEl ? ctx.textEl.value : "",
  });
}

// ---- boot ------------------------------------------------------------------

document.addEventListener("DOMContentLoaded", () => {
  refreshSession();
  initInbox();
  initItem();
});
