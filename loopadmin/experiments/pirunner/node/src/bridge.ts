// Experimental JSON-lines bridge between the loopadmin pirunner experiment
// (Go, test-owned) and one pinned Pi Durable harness. It is a test fixture: it
// opens no listener, reads no credential and calls no provider. Every model
// turn comes from the in-process faux provider, scripted per invocation, and
// every consequential tool asks the Go side's synthetic effect ledger for
// current authority at execution time.
//
// Protocol, one JSON object per line:
//   Go -> bridge  {"id":N,"op":"...",...}       a request
//   bridge -> Go  {"id":N,"ok":true|false,...}  its response
//   bridge -> Go  {"cb":N,"op":"...",...}       a callback the bridge awaits
//   Go -> bridge  {"cb":N,...}                  the callback's answer
//   bridge -> Go  {"event":"...",...}           a counter event (no answer)
//
// The state directory (argv[2]) holds the identity mapping (map/<key>.json),
// written durably before a start is acknowledged, and one Pi SQLite store per
// invocation (inv/<key>.sqlite). Exclusive use of the directory is the Go
// launch owner's job; Pi's built-in storage has no cross-process locking.

import { spawn } from "node:child_process";
import { closeSync, fsyncSync, mkdirSync, openSync, readFileSync, renameSync, writeFileSync, existsSync, readdirSync } from "node:fs";
import { join } from "node:path";
import { createInterface } from "node:readline";

import { awaitWithContext, BACKGROUND_CONTEXT } from "@earendil-works/chord/context";
import type { Context } from "@earendil-works/chord/context";
import { Type } from "@earendil-works/pi-ai";
import { createModels } from "@earendil-works/pi-ai/models";
import { fauxAssistantMessage, fauxProvider, fauxToolCall } from "@earendil-works/pi-ai/providers/faux";
import { createRegistry, defineExtension, defineTool, Harness, hook, ToolTask } from "@earendil-works/pi-durable";
import type { Conversation } from "@earendil-works/pi-durable";
import { openNodeSqliteStorage } from "@earendil-works/pi-durable/storage/sqlite/node";

const ctx = BACKGROUND_CONTEXT;
const stateDir = process.argv[2];
if (!stateDir) {
	process.stderr.write("usage: bridge <state-dir>\n");
	process.exit(2);
}
mkdirSync(join(stateDir, "map"), { recursive: true });
mkdirSync(join(stateDir, "inv"), { recursive: true });

// ---- wire -------------------------------------------------------------------

type Json = null | boolean | number | string | Json[] | { [k: string]: Json };
type Obj = { [k: string]: Json };

function send(msg: object): void {
	process.stdout.write(JSON.stringify(msg) + "\n");
}

const tracing = process.env.PIRUNNER_TRACE === "1";
function trace(...a: unknown[]): void {
	if (tracing) process.stderr.write("trace " + a.map(String).join(" ") + "\n");
}

function event(name: string, fields: Obj): void {
	send({ event: name, ...fields });
}

let nextCb = 1;
const pendingCb = new Map<number, (v: Obj) => void>();

// ask sends a callback to the Go side and waits for its answer. The answer
// never arrives if the Go side decides this process dies first.
function ask(op: string, fields: Obj): Promise<Obj> {
	const cb = nextCb++;
	return new Promise((resolve) => {
		pendingCb.set(cb, resolve);
		send({ cb, op, ...fields });
	});
}

// ---- mapping (fixture store) ------------------------------------------------

type StepTool = { tool: string; args: Obj; callId: string };
type Coverage = { input: boolean; output: boolean; cost: boolean };

type MapRecord = {
	key: string;
	caller: string;
	id: string;
	generation: number;
	descriptor: string; // sha256 of the immutable invocation descriptor
	model: string; // the model this process configured, not the pinned one
	requestId: string;
	storage: string;
	state: "intent" | "admitted" | "abandoned";
	script: StepTool[];
	submissionId?: string;
	conversationId?: string;
	sessionId?: string;
	cancel?: "requested";
	coverage?: Coverage;
	completion?: Completion;
	children?: number[];
};

function mapPath(key: string): string {
	return join(stateDir!, "map", key + ".json");
}

function readMap(key: string): MapRecord | undefined {
	const p = mapPath(key);
	if (!existsSync(p)) return undefined;
	return JSON.parse(readFileSync(p, "utf8")) as MapRecord;
}

// writeMap persists a record with write, fsync, rename and a directory fsync, so
// an acknowledged start survives a kill of this process.
function writeMap(rec: MapRecord): void {
	const p = mapPath(rec.key);
	const tmp = p + ".tmp";
	const fd = openSync(tmp, "w");
	writeFileSync(fd, JSON.stringify(rec));
	fsyncSync(fd);
	closeSync(fd);
	renameSync(tmp, p);
	const dfd = openSync(join(stateDir!, "map"), "r");
	fsyncSync(dfd);
	closeSync(dfd);
}

// ---- faults (test-owned; this fixture is never a managed profile) -------------

const faults = { ignoreCancel: false, modelOverride: "", noUpperHook: false };

// ---- per-invocation harness ---------------------------------------------------

// The completion is the test owner's scripted final answer. Its usage is
// carried opaquely (runner.Usage JSON, absent fields meaning unknown), because
// Pi's own accounting has no unknown value and the faux provider overwrites
// every message's usage with a text-length estimate.
type Completion = { result: Json; usage: Obj };

type Inv = {
	harness: Harness;
	root: Conversation;
	resumed: boolean;
	waiting?: () => void;
};

const open = new Map<string, Inv>();
const gates = new Map<string, () => void>();

// gate parks a tool until the test owner releases it or the task is aborted.
function gate(name: string, tctx: Context): Promise<void> {
	return awaitWithContext(new Promise<void>((resolve) => gates.set(name, resolve)), tctx);
}

function tools(key: string) {
	const effectWrite = defineTool({
		name: "effect_write",
		description: "Apply one consequential synthetic write through the effect ledger",
		parameters: Type.Object({ action: Type.String(), target: Type.String() }),
		replay: "unsafe",
		execute: async (args, api) => {
			const callId = String((api as unknown as { callId?: string }).callId ?? "");
			event("tool-exec", { key, tool: "effect_write", callId, task: String(api.taskId) });
			const r = await ask("effect", { key, action: args.action, target: args.target, task: String(api.taskId) });
			const isError = r.status !== "applied" && r.status !== "duplicate";
			return { output: [{ type: "text", text: JSON.stringify(r) }], isError };
		},
	});
	const safeProbe = defineTool({
		name: "safe_probe",
		description: "A replay-safe read that rechecks current authority at every execution",
		parameters: Type.Object({ step: Type.String(), hold: Type.Optional(Type.Boolean()) }),
		replay: "safe",
		execute: async (args, api, tctx) => {
			const r = await ask("exec-check", { key, tool: "safe_probe", step: args.step, task: String(api.taskId) });
			// tool-exec follows the answered check, so a host that has counted
			// this execution has already recorded the check's verdict.
			event("tool-exec", { key, tool: "safe_probe", step: args.step, task: String(api.taskId) });
			if (r.status !== "allowed") {
				return { output: [{ type: "text", text: "refused: " + String(r.status) }], isError: true };
			}
			if (args.hold) {
				event("tool-held", { key, tool: "safe_probe", step: args.step });
				await gate(key + "/" + args.step, tctx);
			}
			return { output: [{ type: "text", text: "probe " + args.step + " ok" }] };
		},
	});
	const slowTool = defineTool({
		name: "slow_tool",
		description: "A long silent tool that waits for its gate",
		parameters: Type.Object({ step: Type.String() }),
		replay: "unsafe",
		execute: async (args, api, tctx) => {
			event("tool-exec", { key, tool: "slow_tool", step: args.step, task: String(api.taskId) });
			event("tool-held", { key, tool: "slow_tool", step: args.step });
			await gate(key + "/" + args.step, tctx);
			return { output: [{ type: "text", text: "slow " + args.step + " done" }] };
		},
	});
	const spawnChild = defineTool({
		name: "spawn_child",
		description: "Start a background child process that outlives its tool call",
		parameters: Type.Object({ step: Type.String() }),
		replay: "unsafe",
		execute: async (args, api) => {
			event("tool-exec", { key, tool: "spawn_child", step: args.step, task: String(api.taskId) });
			const child = spawn(process.execPath, ["-e", "setInterval(() => {}, 1000)"], { detached: true, stdio: "ignore" });
			child.unref();
			const rec = readMap(key);
			if (rec && child.pid) {
				rec.children = [...(rec.children ?? []), child.pid];
				writeMap(rec);
			}
			event("child", { key, pid: child.pid ?? 0 });
			return { output: [{ type: "text", text: "child " + String(child.pid) }] };
		},
	});
	// await_result parks the run outside the model call until the test owner
	// completes it. Pi serializes every read on one line with an in-flight model
	// call, so a model turn that waited here would block observation entirely.
	// The completion is durable in the mapping record, so a replay after a
	// restart reads it instead of waiting again.
	const awaitResult = defineTool({
		name: "await_result",
		description: "Wait for the test owner to supply this invocation's result",
		parameters: Type.Object({}),
		replay: "safe",
		execute: async (_args, api, tctx) => {
			event("tool-exec", { key, tool: "await_result", task: String(api.taskId) });
			for (;;) {
				if (readMap(key)?.completion) break;
				await awaitWithContext(new Promise<void>((resolve) => {
					const me = open.get(key);
					if (me) me.waiting = resolve;
					else setTimeout(resolve, 50);
				}), tctx);
			}
			return { output: [{ type: "text", text: "result ready" }] };
		},
	});
	return [effectWrite, safeProbe, slowTool, spawnChild, awaitResult];
}

function guard(key: string) {
	return hook(ToolTask, {
		beforeTool: async (call) => {
			event("before-tool", { key, tool: call.name, callId: call.id });
			if (faults.noUpperHook) return undefined;
			const r = await ask("authorize", { key, tool: call.name, callId: call.id });
			return r.status === "allowed" ? undefined : { block: "authority refused: " + String(r.status) };
		},
	});
}

function zeroUsage<M extends { usage: unknown }>(m: M): M {
	m.usage = { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, totalTokens: 0, cost: { input: 0, output: 0, cacheRead: 0, cacheWrite: 0, total: 0 } };
	return m;
}

// openInv opens an invocation's Pi store. resume=false is read-only attach:
// the scheduler is not started, so no task (and no model call) runs.
async function openInv(rec: MapRecord, resume: boolean): Promise<Inv> {
	let inv = open.get(rec.key);
	if (!inv) {
		const faux = fauxProvider({ provider: "faux", models: [{ id: rec.model }] });
		const models = createModels();
		models.setProvider(faux.provider);
		const key = rec.key;
		const step = async (context: { messages: readonly { role: string }[] }) => {
			const turn = context.messages.filter((m) => m.role === "assistant").length;
			event("model-call", { key, turn });
			faux.appendResponses([step as never]);
			const cur = readMap(key) ?? rec;
			const s = cur.script[turn];
			// Tool-call turns report zero usage so the session total is exactly what
			// the test owner supplied at completion; the faux provider would
			// otherwise estimate tokens from text length.
			if (s) {
				return zeroUsage(fauxAssistantMessage(fauxToolCall(s.tool, s.args, { id: s.callId }), { stopReason: "toolUse" }));
			}
			if (!cur.completion) {
				return zeroUsage(fauxAssistantMessage(fauxToolCall("await_result", {}, { id: "await-" + String(turn) }), { stopReason: "toolUse" }));
			}
			const msg = zeroUsage(fauxAssistantMessage(JSON.stringify(cur.completion)));
			return msg;
		};
		faux.setResponses([step as never]);
		const registry = createRegistry();
		const ext = defineExtension({ name: "pirunner", tools: tools(key), hooks: [guard(key)] });
		registry.install(ext);
		const storage = await openNodeSqliteStorage(join(stateDir!, rec.storage));
		const harness = await Harness.open(storage, { models, registry }, ctx);
		const root = await harness.root(ctx, { agent: { model: { provider: "faux", modelId: rec.model }, extensions: [ext] } });
		inv = { harness, root, resumed: false };
		open.set(rec.key, inv);
	}
	if (resume && !inv.resumed) {
		inv.harness.resume();
		inv.resumed = true;
	}
	return inv;
}

async function findSubmission(inv: Inv, requestId: string) {
	const insp = await inv.harness.inspect(ctx);
	return insp.submissions.find((s) => s.requestId === requestId);
}

async function liveTools(inv: Inv): Promise<number> {
	const insp = await inv.harness.inspect(ctx);
	return insp.tasks.filter((t) => (t as unknown as { name?: string }).name === "pi.tool" && (t as unknown as { status?: string }).status !== "terminal").length;
}

function alive(pid: number): boolean {
	try {
		process.kill(pid, 0);
		return true;
	} catch {
		return false;
	}
}

async function actualModel(inv: Inv): Promise<string> {
	const agent = await inv.root.agent(ctx);
	const m = (agent as unknown as { model?: { modelId?: string } }).model;
	return m?.modelId ?? "";
}

// ---- operations ----------------------------------------------------------------

const ops: Record<string, (m: Obj) => Promise<Obj>> = {
	async hello() {
		const pkg = JSON.parse(readFileSync(new URL("../node_modules/@earendil-works/pi-durable/package.json", import.meta.url), "utf8"));
		return { pid: process.pid, node: process.version, pi: pkg.version };
	},

	async faults(m) {
		if (typeof m.ignoreCancel === "boolean") faults.ignoreCancel = m.ignoreCancel;
		if (typeof m.modelOverride === "string") faults.modelOverride = m.modelOverride;
		if (typeof m.noUpperHook === "boolean") faults.noUpperHook = m.noUpperHook;
		return {};
	},

	async start(m) {
		const key = String(m.key);
		if (m.reject === true) throw new BridgeError("rejected", true); // test-owned fault: nothing is written or started
		const prior = readMap(key);
		if (prior) {
			if (prior.descriptor !== m.descriptor) throw new BridgeError("identity-reuse", true);
			if (prior.state === "abandoned") throw new BridgeError("abandoned", true);
			if (prior.state === "intent") await settleIntent(prior, true);
			const rec = readMap(key)!;
			const inv = await openInv(rec, true);
			return { dedupe: true, sessionId: rec.sessionId ?? "", actualModel: await actualModel(inv), submission: rec.submissionId ?? "" };
		}
		const model = faults.modelOverride || String(m.model);
		const rec: MapRecord = {
			key, caller: String(m.caller), id: String(m.invId), generation: Number(m.generation),
			descriptor: String(m.descriptor), model, requestId: key + "/g" + String(m.generation),
			storage: join("inv", key + ".sqlite"), state: "intent",
			script: (m.script as unknown as StepTool[]) ?? [],
		};
		writeMap(rec); // the descriptor and identity are durable before anything runs
		if (m.crash === "after-intent") process.kill(process.pid, "SIGKILL");
		const inv = await openInv(rec, true);
		const sub = await inv.root.submit({ type: "input", content: String(m.input ?? "run"), requestId: rec.requestId }, ctx);
		event("submit", { key, submission: String(sub.id) });
		if (m.crash === "after-submit") process.kill(process.pid, "SIGKILL");
		rec.state = "admitted";
		rec.submissionId = String(sub.id);
		rec.conversationId = String(inv.root.id);
		rec.sessionId = String(inv.root.id);
		writeMap(rec); // the mapping is durable before the acknowledgment
		return { dedupe: false, sessionId: rec.sessionId, actualModel: await actualModel(inv), submission: rec.submissionId };
	},

	async observe(m) {
		return observe(String(m.key), false);
	},

	async reconcile(m) {
		return observe(String(m.key), true);
	},

	async complete(m) {
		const key = String(m.key);
		const rec = readMap(key);
		if (!rec || rec.state !== "admitted") throw new BridgeError("no-run", false);
		const usage = (m.usage ?? {}) as Obj;
		rec.coverage = { input: usage.input_tokens !== undefined, output: usage.output_tokens !== undefined, cost: usage.cost_micros !== undefined };
		rec.completion = { result: m.result as Json, usage };
		writeMap(rec);
		const inv = await openInv(rec, true);
		inv.waiting?.();
		inv.waiting = undefined;
		const sub = await inv.harness.submission(rec.submissionId as never, ctx);
		if (!sub) throw new BridgeError("no-submission", false);
		const settled = await sub.wait(ctx);
		return { status: settled.status };
	},

	async cancel(m) {
		const key = String(m.key);
		const rec = readMap(key);
		if (!rec || rec.state !== "admitted") throw new BridgeError("no-run", false);
		rec.cancel = "requested";
		writeMap(rec);
		if (faults.ignoreCancel) return { requested: true, ignored: true };
		const inv = await openInv(rec, true);
		await inv.root.abort(ctx);
		return { requested: true, ignored: false };
	},

	async release(m) {
		const name = String(m.key) + "/" + String(m.step);
		const g = gates.get(name);
		gates.delete(name);
		g?.();
		return { released: g !== undefined };
	},

	async usage(m) {
		const rec = readMap(String(m.key));
		if (!rec) throw new BridgeError("no-run", false);
		const inv = await openInv(rec, false);
		return { usage: (await inv.harness.usage(ctx)) as unknown as Json };
	},

	async entries(m) {
		const rec = readMap(String(m.key));
		if (!rec) throw new BridgeError("no-run", false);
		const inv = await openInv(rec, false);
		const view = await inv.root.context(ctx);
		const out: Json[] = view.entries.map((e) => {
			const r = e as unknown as { kind?: string; model?: { role?: string; toolName?: string; isError?: boolean; content?: { type: string; text?: string; name?: string }[] }[] };
			const msg = r.model?.[0];
			const parts = Array.isArray(msg?.content) ? msg.content : [];
			const text = parts.filter((c) => c.type === "text").map((c) => c.text ?? "").join("");
			const calls = parts.filter((c) => c.type === "toolCall").map((c) => c.name ?? "");
			return { kind: r.kind ?? "", tool: msg?.toolName ?? calls.join(","), isError: msg?.isError ?? false, text };
		});
		return { entries: out };
	},

	async stats(m) {
		const rec = readMap(String(m.key));
		const mem = process.memoryUsage();
		const out: Obj = { rss: mem.rss, heapUsed: mem.heapUsed };
		if (rec) {
			const inv = await openInv(rec, false);
			const view = await inv.root.context(ctx);
			out.entries = view.entries.length;
		}
		return out;
	},

	async close() {
		for (const inv of open.values()) await inv.harness.close(ctx);
		open.clear();
		setImmediate(() => process.exit(0));
		return {};
	},
};

// settleIntent resolves a record whose start was interrupted before its
// mapping was finished: the store either holds the submission (the launch
// happened; finish the mapping) or it does not (the launch never happened;
// the record is abandoned and can never launch later).
async function settleIntent(rec: MapRecord, resume: boolean): Promise<void> {
	const inv = await openInv(rec, resume);
	const sub = await findSubmission(inv, rec.requestId);
	if (!sub) {
		rec.state = "abandoned";
		writeMap(rec);
		return;
	}
	rec.state = "admitted";
	rec.submissionId = String(sub.id);
	rec.conversationId = String(inv.root.id);
	rec.sessionId = String(inv.root.id);
	writeMap(rec);
}

async function observe(key: string, resume: boolean): Promise<Obj> {
	let rec = readMap(key);
	if (!rec || rec.state === "abandoned") return { state: "absent" };
	if (rec.state === "intent") {
		await settleIntent(rec, resume);
		rec = readMap(key)!;
		if (rec.state === "abandoned") return { state: "absent" };
	}
	const inv = await openInv(rec, resume);
	trace("observe opened");
	const model = await actualModel(inv);
	trace("observe model", model);
	const sub = await inv.harness.submission(rec.submissionId as never, ctx);
	trace("observe submission", sub !== undefined);
	if (!sub) return { state: "unknown", model, why: "submission missing from store" };
	const st = await sub.status(ctx);
	trace("observe status", st.status);
	const children = (rec.children ?? []).filter(alive);
	const base: Obj = { model, session: rec.sessionId ?? "", children: children.length, cancel: rec.cancel ?? "" };
	if (st.status === "queued" || st.status === "placed") {
		return { ...base, state: "running", live: await liveTools(inv) };
	}
	if (st.status === "done") {
		const page = await inv.root.entries({}, 1000, undefined, ctx);
		const answer = page.items.find((e) => String((e as unknown as { id: string }).id) === String(st.answer));
		const msg = (answer as unknown as { model?: { content?: { type: string; text?: string }[] }[] })?.model?.[0];
		const text = (msg?.content ?? []).filter((c) => c.type === "text").map((c) => c.text ?? "").join("");
		const usage = await inv.harness.usage(ctx);
		return { ...base, state: children.length > 0 ? "running" : "finished", result: text, usage: usage as unknown as Json, coverage: (rec.coverage ?? null) as unknown as Json };
	}
	// unanswered: an abort is a stop only once nothing it started still runs.
	const reason = String((st as unknown as { reason?: string }).reason ?? "");
	const live = await liveTools(inv);
	if (reason === "aborted" || rec.cancel === "requested") {
		if (live > 0 || children.length > 0) return { ...base, state: "running", live, reason };
		return { ...base, state: "stopped", reason };
	}
	return { ...base, state: "failed", reason };
}

class BridgeError extends Error {
	constructor(readonly code: string, readonly definite: boolean) {
		super(code);
	}
}

// ---- main loop -------------------------------------------------------------------

const rl = createInterface({ input: process.stdin });
rl.on("line", (line) => {
	if (!line.trim()) return;
	let m: Obj;
	try {
		m = JSON.parse(line) as Obj;
	} catch {
		send({ id: 0, ok: false, error: "bad-json", definite: true });
		return;
	}
	if (typeof m.cb === "number") {
		const r = pendingCb.get(m.cb);
		pendingCb.delete(m.cb);
		r?.(m);
		return;
	}
	const id = m.id;
	const op = ops[String(m.op)];
	if (!op) {
		send({ id, ok: false, error: "unknown-op", definite: true });
		return;
	}
	op(m).then(
		(r) => send({ id, ok: true, ...r }),
		(e: unknown) => {
			if (e instanceof BridgeError) send({ id, ok: false, error: e.code, definite: e.definite });
			else send({ id, ok: false, error: "internal", definite: false, detail: String(e instanceof Error ? e.message : e).slice(0, 300) });
		},
	);
});
rl.on("close", () => process.exit(0));

// Unused import guard for readdirSync (kept for a future list op).
void readdirSync;
