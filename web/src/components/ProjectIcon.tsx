import { Box, Code2, Cpu, Flame, Folder, Layers, Sparkles, Terminal, Workflow, Zap } from 'lucide-react'

/** The icon a project chose in its settings, a folder when it names none. */
export const renderProjectIcon = (iconName: string, size = 15, className = '') => {
  switch (iconName) {
    case 'Terminal': return <Terminal size={size} className={className} />
    case 'Zap': return <Zap size={size} className={className} />
    case 'Flame': return <Flame size={size} className={className} />
    case 'Layers': return <Layers size={size} className={className} />
    case 'Box': return <Box size={size} className={className} />
    case 'Code2': return <Code2 size={size} className={className} />
    case 'Cpu': return <Cpu size={size} className={className} />
    case 'Sparkles': return <Sparkles size={size} className={className} />
    case 'Workflow': return <Workflow size={size} className={className} />
    default: return <Folder size={size} className={className} />
  }
}
