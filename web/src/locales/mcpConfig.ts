export const mcpConfigText = {
  "fr": {
    "title": "Connexion MCP",
    "choose": "Choisissez une seule des deux options dans",
    "merge": "Fusionnez l’entrée sectile avec votre configuration existante, puis redémarrez le moteur.",
    "key": "Remplacez <SECTILE_API_KEY> par votre clé personnelle Sectile. Les deux options accèdent au même serveur et aux mêmes outils.",
    "keys": "Gérer les clés API dans Postes de travail",
    "httpTitle": "Streamable HTTP · connexion distante",
    "stdioTitle": "STDIO · passerelle locale",
    "http": "Le moteur se connecte directement au serveur avec votre clé API. Aucun agent ni processus Sectile local requis.",
    "stdio": "Le moteur lance sectile-agent comme sous-processus et échange via stdin/stdout. Installez le binaire dans le PATH ou remplacez command par son chemin absolu. La passerelle contacte le serveur avec votre clé ; l’application desktop peut rester fermée.",
    "copied": "Configuration copiée.",
    "copyError": "Copie impossible. Sélectionnez le texte de la configuration.",
    "copy": "Copier la configuration",
    "custom": "Configurez le client MCP de votre moteur personnalisé avec l’URL et l’en-tête ci-dessous, ou lancez la passerelle STDIO avec la variable SECTILE_AGENT_TOKEN."
  },
  "en": {
    "title": "MCP connection",
    "choose": "Choose one of the two options in",
    "merge": "Merge the sectile entry into your existing configuration, then restart the engine.",
    "key": "Replace <SECTILE_API_KEY> with your personal Sectile API key. Both options access the same server and tools.",
    "keys": "Manage API keys in Workstations",
    "httpTitle": "Streamable HTTP · remote connection",
    "stdioTitle": "STDIO · local bridge",
    "http": "The engine connects directly to the server with your API key. No local Sectile agent or process is required.",
    "stdio": "The engine starts sectile-agent as a subprocess and exchanges messages over stdin/stdout. Install the binary in PATH or replace command with its absolute path. The bridge contacts the server with your key; the desktop app can remain closed.",
    "copied": "Configuration copied.",
    "copyError": "Copy failed. Select the configuration text to copy it manually.",
    "copy": "Copy configuration",
    "custom": "Configure your custom engine’s MCP client with the URL and header below, or start the STDIO bridge with the SECTILE_AGENT_TOKEN environment variable."
  }
} as const
