// Run with: node --test web/ui_test.js (no browser or dependencies required).
const { test } = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const vm = require('node:vm');
const { webcrypto } = require('node:crypto');

class Element {
  constructor(tag = 'div') { this.tagName = tag; this.children = []; this.listeners = {}; this.dataset = {}; this.value = ''; this.hidden = false; }
  set textContent(v) { this.text = v; this.children = []; }
  get textContent() { return (this.text || '') + this.children.map((c) => c.textContent).join(''); }
  appendChild(n) { this.children.push(n); return n; }
  replaceChildren(...nodes) { this.text = ''; this.children = nodes; }
  addEventListener(name, fn) { this.listeners[name] = fn; }
  removeEventListener(name) { delete this.listeners[name]; }
  setAttribute(name, v) { this[name] = v; }
  querySelector(tag) { return this.children.find((n) => n.tagName === tag); }
  after() {}
}

function setup() {
  const nodes = new Map([...fs.readFileSync(__dirname + '/index.html', 'utf8').matchAll(/id="([^"]+)"/g)].map((m) => [m[1], new Element()]));
  nodes.get('outputRaw').appendChild(new Element('pre'));
  const document = {
    readyState: 'loading', addEventListener() {},
    createElement: (tag) => new Element(tag),
    getElementById: (id) => { assert(nodes.has(id), 'Unknown HTML id: ' + id); return nodes.get(id); },
    querySelector: () => ({ content: '' }),
  };
  const context = vm.createContext({ window: {}, document, TextEncoder, TextDecoder, URLSearchParams, crypto: webcrypto, setTimeout: () => {}, clearTimeout() {} });
  vm.runInContext(fs.readFileSync(__dirname + '/structured.js', 'utf8'), context);
  const source = fs.readFileSync(__dirname + '/app.js', 'utf8');
  // Export closure internals in the test VM only. Do not boot the application.
  vm.runInContext(source.replace(/  if \(document.readyState === 'loading'\) \{[\s\S]*$/, `
    window.test = { state, getEntry, loadDoc, assemble, renderOutput, renderPromptSources, generateSummary, invalidateSummary,
      setAPI: (fn) => { api = fn; }, setMode: (mode) => { outputMode = mode; } };
  })();`), context);
  const api = context.window.test;
  api.state.projectId = 'project-a';
  api.state.stage = 'step-05';
  api.state.pkg = 'P01';
  return { ...api, nodes, structured: context.window.BuildFlowStructured, Element };
}

function answer(ui) {
  const doc = { content: '# Saved answer', hash: 'answer-hash', structured: {
    status: 'valid', artifact: { stage: 'step-05', state: 'ready', blocks: { coding_prompt: 'Code only this package', target: { id: 'P01' } } }, diagnostics: [], footer_start: -1,
  } };
  ui.getEntry('package-output', 'step-05', 'P01').doc = doc;
  ui.nodes.get('outputText').value = doc.content;
  ui.state.assembled = { content: 'current assembly', sources: [] };
  return doc;
}

const configured = { mode: 'free', provider: 'configured-provider', model: 'configured-model', keyPresent: true };

test('valid answers default to Blocks; legacy answers retain Full output and editing', () => {
  const ui = setup();
  const doc = answer(ui);
  ui.renderOutput(true);
  assert.equal(ui.nodes.get('outputBlocks').hidden, false);
  assert.equal(ui.nodes.get('outputText').hidden, true);
  assert.equal(ui.nodes.get('btnCopyCoding').hidden, false);
  doc.structured.status = 'missing';
  ui.renderOutput(true);
  assert.equal(ui.nodes.get('outputPreview').hidden, false);
  assert.equal(ui.nodes.get('btnOutputBlocks').disabled, true);
  assert.equal(ui.nodes.get('btnGenerateSummary').hidden, true);
  ui.setMode('edit');
  ui.renderOutput(false);
  assert.equal(ui.nodes.get('outputText').hidden, false);
  assert.equal(ui.nodes.get('outputText').value, doc.content);
});

test('UTF-8 footer offsets preserve complete Unicode narrative and raw footer', () => {
  const ui = setup();
  const narrative = '# Résumé 🛠\r\n';
  const footer = '## BuildFlow Structured View\nfooter';
  const offset = Buffer.byteLength(narrative);
  const split = ui.structured.split({ content: narrative + footer, structured: { narrative_end: offset, footer_start: offset } });
  assert.equal(split.narrative, narrative);
  assert.equal(split.footer, footer);
});

test('rendered model strings are text and disclosures retain per-target state', () => {
  const ui = setup();
  const parent = new Element();
  const result = { status: 'valid', artifact: { stage: 'step-01', state: 'ready', blocks: { problem: '<img src=x onerror=alert(1)>', goals: [] } } };
  ui.structured.render(parent, result, 'project-a/step-01');
  assert(parent.textContent.includes('<img src=x onerror=alert(1)>'));
  const problem = parent.children[1];
  assert.equal(problem.children[1].tagName, 'pre');
  problem.open = false;
  problem.listeners.toggle();
  ui.structured.render(parent, result, 'project-a/step-01');
  assert.equal(parent.children[1].open, false);
  ui.structured.render(parent, result, 'project-b/step-01');
  assert.equal(parent.children[1].open, true);
});

test('each stage fixture renders its fixed blocks; libraries group by language and tools stay separate', () => {
  const ui = setup();
  for (const stage of ['step-01', 'step-02', 'step-02-5', 'step-03', 'step-04', 'step-05']) {
    const markdown = fs.readFileSync(__dirname + '/../internal/structured/testdata/' + stage + '.md', 'utf8');
    const footer = markdown.slice(markdown.lastIndexOf('<!-- buildflow:view:start -->'));
    const artifact = JSON.parse(footer.match(/```json\s*\n([\s\S]*?)\n```/)[1]);
    const parent = new Element();
    ui.structured.render(parent, { status: 'valid', artifact }, stage);
    assert(parent.textContent.includes('Artifact: ' + artifact.state), stage);
    for (const title of ['Risks', 'Assumptions', 'Open questions', 'Sources']) assert(parent.textContent.includes(title), stage + ': ' + title);
    if (stage === 'step-02-5') {
      const libraries = parent.children.find((n) => n.children[0] && n.children[0].textContent === 'Main libraries by language');
      const tools = parent.children.find((n) => n.children[0] && n.children[0].textContent === 'Suggested tools');
      for (const entry of artifact.blocks.technology.libraries) {
        assert(libraries.children.some((n) => n.tagName === 'h4' && n.textContent === entry.language));
        assert(libraries.textContent.includes(entry.selection_reason));
        for (const location of entry.used_in) assert(libraries.textContent.includes(location));
      }
      for (const tool of artifact.blocks.technology.tools) assert(tools.textContent.includes(tool.category + ' — ' + tool.name + ' — ' + tool.purpose));
    }
  }
});

test('summary sends exact snapshot and blocks, remains separate, caches by hashes', async () => {
  const ui = setup();
  const doc = answer(ui);
  let calls = 0;
  ui.setAPI(async (path, options) => {
    if (path === '/api/llm') return configured;
    if (path.includes('/doc?')) return { exists: true, content: 'EXACT SNAPSHOT' };
    assert.equal(path, '/api/llm/chat');
    calls++;
    assert.equal(options.body.requireFree, true);
    const input = JSON.parse(options.body.messages[1].content);
    assert.equal(input.prompt, 'EXACT SNAPSHOT');
    assert.deepEqual(input.validated_blocks, doc.structured.artifact.blocks);
    return { content: 'Separate summary' };
  });
  await ui.generateSummary();
  assert(ui.nodes.get('generatedSummary').textContent.includes('Separate summary'));
  assert.equal(doc.content, '# Saved answer');
  assert.equal(ui.nodes.get('outputText').value, doc.content);
  await ui.generateSummary();
  assert.equal(calls, 1);
  doc.structured.artifact.blocks.coding_prompt = 'Changed coding prompt';
  ui.invalidateSummary();
  await ui.generateSummary();
  assert.equal(calls, 2);
});

test('missing snapshot uses labelled current assembly; paid or incomplete config opens Settings', async () => {
  for (const cfg of [configured, { ...configured, mode: 'premium' }, { ...configured, model: '' }, { ...configured, keyPresent: false }]) {
    const ui = setup();
    answer(ui);
    let chats = 0;
    ui.setAPI(async (path, options) => {
      if (path === '/api/llm') return cfg;
      if (path.includes('/doc?')) return { exists: false };
      chats++;
      const input = JSON.parse(options.body.messages[1].content);
      assert.equal(input.prompt, 'current assembly');
      assert.match(input.prompt_source, /may differ from the original/);
      return { content: 'Summary' };
    });
    await ui.generateSummary();
    const allowed = cfg.mode === 'free' && cfg.model && cfg.keyPresent;
    assert.equal(chats, allowed ? 1 : 0);
    if (!allowed) assert(ui.nodes.get('generatedSummary').textContent.includes('Open Settings'));
  }
});

test('late summaries after edits or navigation are ignored; errors expose manual retry', async () => {
  for (const change of ['edit', 'project', 'package']) {
    const ui = setup();
    answer(ui);
    let resolveChat;
    let started;
    const pending = new Promise((r) => { started = r; });
    ui.setAPI(async (path) => {
      if (path === '/api/llm') return configured;
      if (path.includes('/doc?')) return { exists: false };
      started();
      return new Promise((r) => { resolveChat = r; });
    });
    const work = ui.generateSummary();
    await pending;
    if (change === 'project') ui.state.projectId = 'project-b';
    if (change === 'package') ui.state.pkg = 'P02';
    ui.invalidateSummary();
    resolveChat({ content: 'STALE RESULT' });
    await work;
    assert.equal(ui.nodes.get('generatedSummary').hidden, true);
    assert(!ui.nodes.get('generatedSummary').textContent.includes('STALE RESULT'));
  }
  const ui = setup();
  answer(ui);
  ui.setAPI(async (path) => {
    if (path === '/api/llm') return configured;
    if (path.includes('/doc?')) return { exists: false };
    throw new Error('Provider rate limit');
  });
  await ui.generateSummary();
  assert(ui.nodes.get('generatedSummary').textContent.includes('Provider rate limit'));
  assert(ui.nodes.get('generatedSummary').textContent.includes('Retry summary'));
});

test('late document and assembly responses do not populate a different project/package', async () => {
  const ui = setup();
  let resolve;
  ui.setAPI(() => new Promise((r) => { resolve = r; }));
  const load = ui.loadDoc({ kind: 'package-output', stage: 'step-05', pkg: 'P01' });
  ui.state.projectId = 'project-b';
  resolve({ content: 'old project answer' });
  await load;
  assert.equal(Object.keys(ui.state.docs).length, 0);
  const assemble = ui.assemble(false);
  ui.state.pkg = 'P02';
  resolve({ content: 'old package prompt' });
  await assemble;
  assert.equal(ui.state.assembled, null);
  assert.equal(ui.nodes.get('promptText').value, '');
});
