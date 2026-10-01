// A conversation draws each tool call from the arguments Claude sent with it,
// the way a reader thinks of it: an edit as a diff, a command as a command, a
// todo list as a checklist. The arguments are engine output, untrusted: the
// model below holds plain strings only, and the DOM is built from text nodes.
//
// toolCard returns {name, target, note, open, body}. body is one of:
//   {type:'diff', hunks:[[{sign,text}]], more}
//   {type:'code', text}
//   {type:'todos', items:[{status,text}]}
//   null, for a call that says everything on its summary line.

// What a card shows of a long text before it says how much it left out.
export const CARD_LINES = 400
export const RESULT_LINES = 200

// Tools whose answer only confirms what the card already shows: their result
// is shown when it is an error, and only then.
const QUIET_RESULTS = new Set(['Edit', 'MultiEdit', 'Write', 'NotebookEdit', 'TodoWrite'])

// What a tool answered, bounded for display. truncated says the agent already
// cut it; more counts the lines the card leaves out on top of that.
export function toolResult(event) {
  const all = lines(event?.text)
  return { lines: all.slice(0, RESULT_LINES), more: Math.max(0, all.length - RESULT_LINES), truncated: !!event?.truncated, error: !!event?.error }
}

const str = value => typeof value === 'string' ? value : ''

// A path inside the conversation's directory is shown relative to it.
export function shortPath(path, root) {
  const value = str(path)
  const base = str(root).replace(/\/+$/, '')
  if (base && value.startsWith(base + '/')) return value.slice(base.length + 1)
  return value
}

const lines = text => str(text).split('\n')

// One hunk of an edit: the lines both sides share at the start and the end are
// context, what is left between them is removed then added.
export function editHunk(oldText, newText) {
  const before = lines(oldText), after = lines(newText)
  let start = 0
  while (start < before.length && start < after.length && before[start] === after[start]) start++
  let end = 0
  while (end < before.length - start && end < after.length - start && before[before.length - 1 - end] === after[after.length - 1 - end]) end++
  return [
    ...before.slice(0, start).map(text => ({ sign: ' ', text })),
    ...before.slice(start, before.length - end).map(text => ({ sign: '-', text })),
    ...after.slice(start, after.length - end).map(text => ({ sign: '+', text })),
    ...before.slice(before.length - end).map(text => ({ sign: ' ', text })),
  ]
}

// Cuts hunks to CARD_LINES lines in all, and counts what was cut.
function bounded(hunks) {
  let left = CARD_LINES, more = 0
  const kept = []
  for (const hunk of hunks) {
    if (left <= 0) { more += hunk.length; continue }
    kept.push(hunk.slice(0, left))
    more += Math.max(0, hunk.length - left)
    left -= hunk.length
  }
  return { type: 'diff', hunks: kept, more }
}

const counts = hunks => {
  let added = 0, removed = 0
  for (const hunk of hunks) for (const line of hunk) {
    if (line.sign === '+') added++
    else if (line.sign === '-') removed++
  }
  return `+${added} −${removed}`
}

const TODO_STATUSES = new Set(['pending', 'in_progress', 'completed'])

function parse(input) {
  if (input && typeof input === 'object' && !Array.isArray(input)) return input
  return null
}

// result is the tool_result event that answered this call, if any; pending says
// Claude is still working and the call has not been answered yet.
export function toolCard(event, root = '', { result = null, pending = false } = {}) {
  const card = baseCard(event, root)
  const quiet = QUIET_RESULTS.has(str(event?.tool) || str(event?.text))
  if (result && (!quiet || result.error)) card.result = toolResult(result)
  card.failed = !!result?.error
  card.pending = pending && !result
  return card
}

function baseCard(event, root) {
  const name = str(event?.text) || str(event?.tool) || 'Tool'
  const args = parse(event?.input)
  const fallback = () => ({ name, target: str(event?.detail), note: '', open: false, body: event?.input !== undefined ? { type: 'code', text: JSON.stringify(event.input, null, 2) } : str(event?.detail) ? { type: 'code', text: str(event.detail) } : null })
  if (!args) return fallback()
  const path = shortPath(args.file_path || args.notebook_path || args.path, root)
  switch (str(event.tool) || name) {
    case 'Edit': {
      const hunks = [editHunk(args.old_string, args.new_string)]
      return { name, target: path, note: counts(hunks) + (args.replace_all ? ' · every occurrence' : ''), open: false, body: bounded(hunks) }
    }
    case 'MultiEdit': {
      const edits = Array.isArray(args.edits) ? args.edits.filter(edit => edit && typeof edit === 'object') : []
      const hunks = edits.map(edit => editHunk(edit.old_string, edit.new_string))
      return { name, target: path, note: counts(hunks) + ` · ${hunks.length} edit${hunks.length === 1 ? '' : 's'}`, open: false, body: bounded(hunks) }
    }
    case 'Write': {
      const hunks = [lines(args.content).map(text => ({ sign: '+', text }))]
      return { name, target: path, note: `${hunks[0].length} line${hunks[0].length === 1 ? '' : 's'}`, open: false, body: bounded(hunks) }
    }
    case 'Read': {
      const offset = Number.isInteger(args.offset) ? args.offset : null
      const limit = Number.isInteger(args.limit) ? args.limit : null
      const note = offset !== null || limit !== null ? `lines ${offset ?? 1}-${limit !== null ? (offset ?? 1) + limit - 1 : 'end'}` : ''
      return { name, target: path, note, open: false, body: null }
    }
    case 'Bash': {
      const command = str(args.command)
      return { name, target: str(args.description) || command.split('\n')[0], note: args.run_in_background ? 'in background' : '', open: false, body: command ? { type: 'code', text: '$ ' + command } : null }
    }
    case 'Grep':
      return { name, target: str(args.pattern), note: [path, str(args.glob), str(args.type)].filter(Boolean).join(' · '), open: false, body: null }
    case 'Glob':
      return { name, target: str(args.pattern), note: path, open: false, body: null }
    case 'WebFetch':
      return { name, target: str(args.url), note: '', open: false, body: str(args.prompt) ? { type: 'code', text: str(args.prompt) } : null }
    case 'WebSearch':
      return { name, target: str(args.query), note: '', open: false, body: null }
    case 'Task':
    case 'Agent':
      return { name, target: [str(args.subagent_type), str(args.description)].filter(Boolean).join(' · '), note: '', open: false, body: str(args.prompt) ? { type: 'code', text: str(args.prompt) } : null }
    case 'TodoWrite': {
      const items = (Array.isArray(args.todos) ? args.todos : [])
        .filter(todo => todo && typeof todo === 'object')
        .map(todo => ({ status: TODO_STATUSES.has(todo.status) ? todo.status : 'pending', text: str(todo.content) }))
      const done = items.filter(item => item.status === 'completed').length
      return { name: 'Todos', target: `${done} of ${items.length} done`, note: '', open: true, body: { type: 'todos', items } }
    }
  }
  return fallback()
}

// renderToolCard draws a card with text nodes only. A card with a body is a
// <details>, so the reader opens what interests them; one without is a line.
export function renderToolCard(card, { document = globalThis.document } = {}) {
  const el = (tag, className, text) => {
    const node = document.createElement(tag)
    if (className) node.className = className
    if (text !== undefined) node.textContent = text
    return node
  }
  const expandable = !!(card.body || card.result)
  const node = el(expandable ? 'details' : 'div', 'conversation-event conversation-tool tool-card' + (expandable ? '' : ' tool-card-line') + (card.failed ? ' tool-card-failed' : ''))
  const summary = el(expandable ? 'summary' : 'div', 'tool-card-summary')
  summary.append(el('span', 'tool-card-name', card.name))
  if (card.target) summary.append(el('span', 'tool-card-target', card.target))
  const status = card.failed ? 'failed' : card.pending ? 'running…' : ''
  if (card.note || status) {
    const note = el('span', 'tool-card-note', card.note)
    if (status) note.append(el('span', 'tool-card-status', (card.note ? ' · ' : '') + status))
    summary.append(note)
  }
  node.append(summary)
  const body = card.body
  if (body?.type === 'code') node.append(el('pre', 'tool-card-code', body.text))
  if (body?.type === 'diff') {
    const pre = el('pre', 'tool-card-diff')
    body.hunks.forEach((hunk, index) => {
      if (index) pre.append(el('span', 'diff-hunk', '…\n'))
      for (const line of hunk) pre.append(el('span', line.sign === '+' ? 'diff-add' : line.sign === '-' ? 'diff-delete' : 'diff-line', line.sign + ' ' + line.text + '\n'))
    })
    if (body.more) pre.append(el('span', 'diff-hunk', `… ${body.more} more line${body.more === 1 ? '' : 's'}\n`))
    node.append(pre)
  }
  if (body?.type === 'todos') {
    const list = el('ul', 'tool-card-todos')
    for (const item of body.items) {
      const row = el('li', 'tool-card-todo tool-card-todo-' + item.status)
      const box = el('input'); box.type = 'checkbox'; box.disabled = true; box.checked = item.status === 'completed'
      box.setAttribute('aria-label', item.status === 'completed' ? 'Done' : item.status === 'in_progress' ? 'In progress' : 'To do')
      row.append(box, el('span', '', item.text))
      list.append(row)
    }
    node.append(list)
  }
  if (card.result) {
    const { lines: shown, more, truncated, error } = card.result
    const pre = el('pre', 'tool-card-result' + (error ? ' tool-card-result-error' : ''), shown.join('\n'))
    pre.setAttribute('aria-label', error ? 'Tool error' : 'Tool output')
    if (more || truncated) pre.append(el('span', 'diff-hunk', '\n… ' + (more ? `${more} more line${more === 1 ? '' : 's'}` : 'output cut') + (truncated && more ? ', output cut' : '')))
    node.append(pre)
  }
  return node
}
