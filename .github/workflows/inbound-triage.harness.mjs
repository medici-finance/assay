#!/usr/bin/env node
// Fixture harness for .github/workflows/inbound-triage.yml
//
// This does NOT re-implement the workflow's decision logic — it extracts the
// literal `if:` job-skip expression and the literal `script:` body from the
// committed YAML and executes them, offline, against mocked GitHub API
// fixtures. If the shipped logic changes (or breaks), this harness runs the
// new text and the assertions below catch the drift — there is nothing to
// keep "in sync" by hand.
//
// Run:   node .github/workflows/inbound-triage.harness.mjs
// Exit:  0 on all assertions + the mutation-sensitivity check passing;
//        1 (with a printed diff of failures) otherwise.
//
// No dependencies beyond Node's standard library (fs, url, assert).

import { readFileSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { dirname, join } from 'node:path';

const __dirname = dirname(fileURLToPath(import.meta.url));
const WORKFLOW_PATH = join(__dirname, 'inbound-triage.yml');

// ---------------------------------------------------------------------------
// 1. Extraction: pull the real `if:` expression and `script:` body out of the
//    committed YAML. Both are YAML block scalars; we dedent by the indent of
//    the first non-blank content line, which is all either style needs here.
// ---------------------------------------------------------------------------

function extractBlockAfter(lines, keyLineRegex) {
  const idx = lines.findIndex((l) => keyLineRegex.test(l));
  if (idx === -1) {
    throw new Error(`extraction key not found: ${keyLineRegex}`);
  }
  const keyIndent = lines[idx].match(/^(\s*)/)[1].length;
  const out = [];
  let baseIndent = null;
  for (let i = idx + 1; i < lines.length; i++) {
    const line = lines[i];
    if (line.trim() === '') {
      out.push('');
      continue;
    }
    const indent = line.match(/^(\s*)/)[1].length;
    if (indent <= keyIndent) break;
    if (baseIndent === null) baseIndent = indent;
    out.push(line.slice(baseIndent));
  }
  // trim trailing blank lines
  while (out.length && out[out.length - 1] === '') out.pop();
  return out.join('\n');
}

function loadWorkflowParts(workflowText) {
  const lines = workflowText.split('\n');

  // The job-skip gate: a YAML folded scalar (`if: >-`) of `&&`-joined
  // `!=` comparisons against github.event.pull_request.* fields. Folding
  // joins the lines with spaces per YAML semantics; that's all we need here.
  const ifBlock = extractBlockAfter(lines, /^\s*if:\s*>-\s*$/)
    .split('\n')
    .join(' ')
    .replace(/\s+/g, ' ')
    .trim();

  const scriptBody = extractBlockAfter(lines, /^\s*script:\s*\|\s*$/);

  // env: defaults, e.g.  ENFORCE_ISSUE_FIRST: "false"
  const envBlock = extractBlockAfter(lines, /^env:\s*$/);
  const env = {};
  for (const line of envBlock.split('\n')) {
    const m = line.match(/^\s*([A-Z_]+):\s*"([^"]*)"/);
    if (m) env[m[1]] = m[2];
  }

  return { ifBlock, scriptBody, env };
}

// Compile the extracted `if:` expression into a callable predicate. The
// expression only ever references github.event.pull_request.* — we rewrite
// those paths onto a local `pr` binding and hand the rest (`!=`, `&&`,
// string literals) straight to the JS engine, since Actions expression
// syntax for this narrow subset is already valid JS.
function compileSkipGate(ifExpr) {
  const rewritten = ifExpr
    .replaceAll('github.event.pull_request.', 'pr.');
  if (/[^a-zA-Z0-9_.'"!=&\s]/.test(rewritten)) {
    throw new Error(`if: expression has unexpected tokens after rewrite: ${rewritten}`);
  }
  // eslint-disable-next-line no-new-func
  const fn = new Function('pr', `return (${rewritten});`);
  return (pr) => Boolean(fn(pr));
}

// Compile the extracted `script:` body into an invocable async function
// matching actions/github-script's own call convention: (github, context).
// The script also reads `process.env` directly (as it does under the real
// action), so callers mutate real process.env before invoking.
function compileScript(scriptBody) {
  const AsyncFunction = Object.getPrototypeOf(async () => {}).constructor;
  return new AsyncFunction('github', 'context', scriptBody);
}

// ---------------------------------------------------------------------------
// 2. Mock GitHub API surface. Tracks calls in `state` for assertions.
// ---------------------------------------------------------------------------

function freshState() {
  return {
    comments: [],       // { issue_number, body }
    labelCalls: [],      // { issue_number, labels }
    updateCalls: [],      // { pull_number, state }
    issues: new Map(),    // number -> { state, pull_request }
    openPRs: [],       // [{ user: { login } }]
  };
}

function makeGithub(state) {
  return {
    paginate: async (fn, params) => fn(params),
    rest: {
      issues: {
        listComments: async (params) =>
          state.comments.filter((c) => c.issue_number === params.issue_number),
        createComment: async (params) => {
          state.comments.push({ issue_number: params.issue_number, body: params.body });
          return { data: {} };
        },
        addLabels: async (params) => {
          state.labelCalls.push({ issue_number: params.issue_number, labels: [...params.labels] });
          return { data: {} };
        },
        get: async (params) => {
          const issue = state.issues.get(params.issue_number);
          if (!issue) {
            const err = new Error('Not Found');
            err.status = 404;
            throw err;
          }
          return { data: issue };
        },
      },
      pulls: {
        list: async () => state.openPRs,
        update: async (params) => {
          state.updateCalls.push({ pull_number: params.pull_number, state: params.state });
          return { data: {} };
        },
      },
    },
  };
}

function makeContext(pr) {
  return {
    repo: { owner: 'acme', repo: 'demo' },
    payload: { pull_request: pr },
  };
}

function makePR(overrides = {}) {
  return {
    number: 501,
    title: 'Add a widget',
    body: '',
    user: { login: 'driveby-dev', type: 'User' },
    author_association: 'NONE',
    created_at: new Date().toISOString(),
    ...overrides,
  };
}

function daysAgoISO(days) {
  return new Date(Date.now() - days * 86400000).toISOString();
}

async function withEnv(overrides, fn) {
  const prior = {};
  for (const k of Object.keys(overrides)) prior[k] = process.env[k];
  Object.assign(process.env, overrides);
  try {
    return await fn();
  } finally {
    for (const k of Object.keys(overrides)) {
      if (prior[k] === undefined) delete process.env[k];
      else process.env[k] = prior[k];
    }
  }
}

// ---------------------------------------------------------------------------
// 3. Assertions
// ---------------------------------------------------------------------------

const results = [];
function check(name, cond, detail = '') {
  results.push({ name, pass: Boolean(cond), detail });
}

function issueFirstComments(state) {
  return state.comments.filter((c) => c.body.includes('<!-- inbound-triage:issue-first -->'));
}
function concurrencyComments(state) {
  return state.comments.filter((c) => c.body.includes('<!-- inbound-triage:concurrency -->'));
}

async function runSuite(skipGate, runScript, defaultEnv) {
  // --- job-skip gate (5 assertions) ---------------------------------------
  check(
    '1. OWNER author is skipped by the job-level gate',
    skipGate(makePR({ author_association: 'OWNER' })) === false,
  );
  check(
    '2. MEMBER author is skipped by the job-level gate',
    skipGate(makePR({ author_association: 'MEMBER' })) === false,
  );
  check(
    '3. COLLABORATOR author is skipped by the job-level gate',
    skipGate(makePR({ author_association: 'COLLABORATOR' })) === false,
  );
  check(
    '4. Bot-authored PR is skipped regardless of author_association',
    skipGate(makePR({ author_association: 'NONE', user: { login: 'some-bot[bot]', type: 'Bot' } })) === false,
  );
  check(
    '5. Untrusted human author (NONE) reaches the gate (not skipped)',
    skipGate(makePR({ author_association: 'NONE' })) === true,
  );

  // --- issue-first gate (4 assertions) ------------------------------------
  {
    const state = freshState();
    const pr = makePR({ number: 601, body: 'Adds a widget, no issue reference.' });
    await withEnv(defaultEnv, () => runScript(makeGithub(state), makeContext(pr)));
    check(
      '6. Unlinked untrusted PR: advisory comment + label fire, PR NOT closed',
      issueFirstComments(state).length === 1 &&
        state.labelCalls.some((l) => l.labels.includes('needs-issue-link')) &&
        state.updateCalls.length === 0,
      JSON.stringify({ comments: state.comments.length, labels: state.labelCalls, updates: state.updateCalls }),
    );
  }
  {
    const state = freshState();
    state.issues.set(42, { state: 'open', pull_request: undefined });
    const pr = makePR({ number: 602, body: 'Fixes the widget, closes #42.' });
    await withEnv(defaultEnv, () => runScript(makeGithub(state), makeContext(pr)));
    check(
      '7. PR linking an OPEN issue: issue-first advisory does NOT fire',
      issueFirstComments(state).length === 0,
      JSON.stringify(state.comments),
    );
  }
  {
    const state = freshState();
    state.issues.set(43, { state: 'closed', pull_request: undefined });
    const pr = makePR({ number: 603, body: 'Related to #43 (already closed).' });
    await withEnv(defaultEnv, () => runScript(makeGithub(state), makeContext(pr)));
    check(
      '8. PR referencing only a CLOSED issue: gate still fires (closed ref does not satisfy linkage)',
      issueFirstComments(state).length === 1,
      JSON.stringify(state.comments),
    );
  }
  {
    const state = freshState();
    const pr = makePR({ number: 604, body: 'No issue reference.' });
    await withEnv(defaultEnv, () => runScript(makeGithub(state), makeContext(pr)));
    await withEnv(defaultEnv, () => runScript(makeGithub(state), makeContext(pr))); // simulate "synchronize" re-fire
    check(
      '9. Idempotency: a second run against the same PR does not duplicate the advisory comment',
      issueFirstComments(state).length === 1,
      JSON.stringify(state.comments.length),
    );
  }

  // --- concurrency gate (2 assertions) -------------------------------------
  {
    const state = freshState();
    const author = 'driveby-dev';
    state.openPRs = [
      { user: { login: author } },
      { user: { login: author } },
      { user: { login: author } },
      { user: { login: author } },
    ]; // 4 open PRs by this author, default MAX_OPEN_PR_PER_AUTHOR = 3
    const pr = makePR({ number: 605, user: { login: author, type: 'User' }, body: 'closes #900' });
    state.issues.set(900, { state: 'open', pull_request: undefined });
    await withEnv(defaultEnv, () => runScript(makeGithub(state), makeContext(pr)));
    check(
      '10. Author over the open-PR limit (4 > 3): concurrency advisory fires',
      concurrencyComments(state).length === 1 &&
        state.labelCalls.some((l) => l.labels.includes('too-many-open-prs')),
      JSON.stringify({ comments: state.comments.length, labels: state.labelCalls }),
    );
  }
  {
    const state = freshState();
    const author = 'driveby-dev';
    state.openPRs = [{ user: { login: author } }, { user: { login: author } }, { user: { login: author } }];
    const pr = makePR({ number: 606, user: { login: author, type: 'User' }, body: 'closes #901' });
    state.issues.set(901, { state: 'open', pull_request: undefined });
    await withEnv(defaultEnv, () => runScript(makeGithub(state), makeContext(pr)));
    check(
      '11. Author AT the open-PR limit (3 == 3, not over): concurrency advisory does not fire',
      concurrencyComments(state).length === 0,
      JSON.stringify(state.comments),
    );
  }

  // --- enforcement-off safety net (bonus, not counted in the 11 above but
  //     directly proves the PR's "OFF by default" trust-model claim) --------
  {
    const state = freshState();
    const pr = makePR({ number: 607, body: 'No issue reference.', created_at: daysAgoISO(30) });
    // GRACE_DAYS default is 7; this PR is 30 days old and unlinked. With
    // ENFORCE_ISSUE_FIRST left at its shipped default ("false") the PR must
    // NOT be auto-closed.
    await withEnv(defaultEnv, () => runScript(makeGithub(state), makeContext(pr)));
    check(
      'bonus. ENFORCE_ISSUE_FIRST default "false": a stale unlinked PR is commented on but never closed',
      state.updateCalls.length === 0,
      JSON.stringify(state.updateCalls),
    );
  }
}

// ---------------------------------------------------------------------------
// 4. Run against the real, committed workflow text.
// ---------------------------------------------------------------------------

function report(results, label) {
  let failed = 0;
  for (const r of results) {
    const mark = r.pass ? 'PASS' : 'FAIL';
    if (!r.pass) failed++;
    console.log(`  [${mark}] ${r.name}${r.pass ? '' : `  -- ${r.detail}`}`);
  }
  const core = results.filter((r) => /^\d/.test(r.name));
  const coreFailed = core.filter((r) => !r.pass).length;
  console.log(`${label}: ${core.length - coreFailed}/${core.length} core assertions passed (+${results.length - core.length} bonus)`);
  return failed;
}

async function main() {
  const workflowText = readFileSync(WORKFLOW_PATH, 'utf8');
  const { ifBlock, scriptBody, env } = loadWorkflowParts(workflowText);

  console.log(`Extracted job-skip gate: ${ifBlock}`);
  console.log(`Extracted env defaults: ${JSON.stringify(env)}`);
  console.log('');
  console.log('=== Suite: real inbound-triage.yml (expected: all pass) ===');

  const skipGate = compileSkipGate(ifBlock);
  const scriptFn = compileScript(scriptBody);
  const runScript = (github, context) => scriptFn(github, context);

  results.length = 0;
  await runSuite(skipGate, runScript, env);
  const failedReal = report(results, 'real workflow');

  // -------------------------------------------------------------------------
  // 5. Fail-first / mutation-sensitivity proof: corrupt the shipped decision
  //    logic (flip the linkage check) and re-run the same suite against the
  //    mutant. Assertion 7 ("a linked OPEN issue suppresses the advisory")
  //    must now FAIL — proving this harness actually goes red when the logic
  //    breaks, not just green by construction.
  // -------------------------------------------------------------------------
  console.log('');
  console.log('=== Mutation check: linksOpenIssue() gate inverted (expected: assertion 7 goes red) ===');

  const mutatedScriptBody = scriptBody.replace(
    'if (!(await linksOpenIssue())) {',
    'if (await linksOpenIssue()) { /* MUTATED: gate inverted for fail-first proof */',
  );
  if (mutatedScriptBody === scriptBody) {
    throw new Error('mutation target string not found — workflow logic drifted, update the harness');
  }
  const mutatedScriptFn = compileScript(mutatedScriptBody);
  const mutatedRunScript = (github, context) => mutatedScriptFn(github, context);

  results.length = 0;
  await runSuite(skipGate, mutatedRunScript, env);
  const mutantFailures = results.filter((r) => !r.pass);
  const sawExpectedBreak = mutantFailures.some((r) => r.name.startsWith('7.'));
  report(results, 'mutated workflow');
  console.log(
    sawExpectedBreak
      ? '  mutation-sensitivity: CONFIRMED (assertion 7 went red as expected — harness is not vacuously green)'
      : '  mutation-sensitivity: NOT CONFIRMED (expected assertion 7 to fail against the mutant, it did not)',
  );

  const overallFail = failedReal > 0 || !sawExpectedBreak;
  console.log('');
  console.log(overallFail ? 'RESULT: FAIL' : 'RESULT: PASS (11/11 real-workflow assertions + mutation-sensitivity check)');
  process.exitCode = overallFail ? 1 : 0;
}

main().catch((err) => {
  console.error('harness crashed:', err);
  process.exitCode = 1;
});
