/* Fixed stage views over server-validated schemas. Never parse headings as data. */
'use strict';
(function () {
  const common = ['risks', 'assumptions', 'open_questions', 'sources'];
  const fields = {
    'step-01': ['problem', 'goals', 'non_goals', 'scenarios', 'requirements', 'constraints', 'integrations', 'data_state', 'options', 'evaluation_criteria'],
    'step-02': ['decision', 'alternatives', 'requirements', 'constraints', 'design', 'scope', 'delivery_plan', 'settled_decisions'],
    'step-02-5': ['components', 'contracts', 'technology.stack', 'technology.tools', 'technology.libraries', 'technology.services', 'technology.environment', 'technology.commands', 'technology.security', 'technology.observability', 'costs', 'system_context', 'rejected_technologies'],
    'step-03': ['requirements', 'constraints', 'decisions', 'scope', 'implementation_context', 'modules', 'contracts', 'algorithms', 'error_cases', 'concurrency', 'state_machines', 'surfaces', 'observability', 'test_plan', 'acceptance', 'requirement_coverage', 'build_batches', 'deliberately_excluded', 'design_changes'],
    'step-04': ['packages', 'build_batches', 'integration_points', 'deliberately_not_created'],
    'step-05': ['target', 'scope', 'files', 'interfaces_consumed', 'interfaces_produced', 'implementation_instructions', 'planned_checks', 'acceptance', 'blockers', 'coding_prompt'],
  };
  const names = { libraries: 'Main libraries by language', tools: 'Suggested tools', system_context: 'Reusable context', implementation_context: 'Effective context', build_batches: 'Build order', packages: 'Work packages and prerequisites', scope: 'Boundaries', alternatives: 'Rejected alternatives', planned_checks: 'Planned checks (not executed)' };
  const openStates = new Map();
  function node(tag, text) {
    const n = document.createElement(tag);
    if (text !== undefined) n.textContent = text;
    return n;
  }
  function label(key) {
    return names[key] || key.replace(/_/g, ' ').replace(/^./, (c) => c.toUpperCase());
  }
  function disclosure(title, key, initiallyOpen) {
    const d = node('details');
    d.appendChild(node('summary', title));
    remember(d, key, initiallyOpen);
    return d;
  }
  function remember(d, key, initiallyOpen = false) {
    if (d._rememberToggle) d.removeEventListener('toggle', d._rememberToggle);
    d.open = openStates.has(key) ? openStates.get(key) : initiallyOpen;
    d._rememberToggle = () => openStates.set(key, d.open);
    d.addEventListener('toggle', d._rememberToggle);
  }
  function value(parent, data, key) {
    if (data === null || data === undefined) { parent.appendChild(node('p', 'Not specified')); return; }
    if (typeof data !== 'object') {
      const p = node('pre', String(data));
      p.className = 'block-text';
      parent.appendChild(p);
    } else if (Array.isArray(data)) {
      if (!data.length) parent.appendChild(node('p', 'None recorded'));
      data.forEach((item, i) => {
        const id = item && typeof item === 'object' && !Array.isArray(item) && item.id;
        const title = item && (item.title || item.name || item.statement || item.question || item.choice);
        if (typeof item === 'object' && item !== null) {
          const d = disclosure([id, title].filter(Boolean).join(' — ') || 'Item ' + (i + 1), key + '/' + (id || i), false);
          value(d, item, key + '/' + (id || i));
          parent.appendChild(d);
        } else value(parent, item, key + '/' + i);
      });
    } else {
      const dl = node('dl');
      Object.entries(data).forEach(([k, v]) => {
        dl.appendChild(node('dt', label(k)));
        const dd = node('dd');
        value(dd, v, key + '/' + k);
        dl.appendChild(dd);
      });
      parent.appendChild(dl);
    }
  }
  function libraries(parent, entries, key) {
    const groups = new Map();
    entries.forEach((entry) => {
      if (!groups.has(entry.language)) groups.set(entry.language, []);
      groups.get(entry.language).push(entry);
    });
    groups.forEach((items, language) => {
      parent.appendChild(node('h4', language));
      items.forEach((item) => {
        const d = disclosure(item.name + ' — ' + item.selection_reason + '; ' + item.used_in.join(', '), key + '/' + item.id, false);
        value(d, item, key + '/' + item.id);
        parent.appendChild(d);
      });
    });
    if (!entries.length) parent.appendChild(node('p', 'None recorded'));
  }
  function render(parent, result, targetKey) {
    parent.replaceChildren();
    if (!result || result.status !== 'valid' || !result.artifact) return;
    const artifact = result.artifact;
    parent.appendChild(node('p', 'Artifact: ' + artifact.state + (artifact.stage === 'step-05' ? ' — coding prompt and planned checks only' : '')));
    [...(fields[artifact.stage] || []), ...common].forEach((path) => {
      const data = path.split('.').reduce((v, k) => v && v[k], artifact.blocks);
      if (data === undefined) return;
      const key = targetKey + '/' + path;
      const d = disclosure(label(path.split('.').pop()), key, true);
      if (path === 'technology.libraries') libraries(d, data, key);
      else if (path === 'technology.tools') {
        data.forEach((tool) => {
          const item = disclosure(tool.category + ' — ' + tool.name + ' — ' + tool.purpose, key + '/' + tool.id, false);
          value(item, tool, key + '/' + tool.id);
          d.appendChild(item);
        });
        if (!data.length) d.appendChild(node('p', 'None recorded'));
      } else value(d, data, key);
      parent.appendChild(d);
    });
  }
  // Parser offsets count UTF-8 bytes, while JS string offsets count UTF-16 units.
  function split(doc) {
    const r = doc.structured;
    if (!r || r.footer_start < 0) return { narrative: doc.content, footer: '' };
    const bytes = new TextEncoder().encode(doc.content);
    const decode = (a, b) => new TextDecoder().decode(bytes.slice(a, b));
    return { narrative: decode(0, r.narrative_end), footer: decode(r.footer_start) };
  }
  window.BuildFlowStructured = { render, split, remember };
})();
