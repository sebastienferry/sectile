package agent

// os.Root confines Windows reparse points; identity checks reject substitutions
// before reading any content. Windows has no Unix FIFO open behavior.
const agentProfileOpenFlags = 0
