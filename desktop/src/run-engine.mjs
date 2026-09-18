// Le moteur d'un run, tel qu'il a réellement tourné. L'agent le rapporte une
// fois la ligne de commande construite, donc un run peut n'en porter aucun :
// c'est le cas de tous ceux enregistrés avant que Sectile ne le retienne, et
// de ceux dont le moteur n'accepte pas de modèle.
export function runEngine(run){
 return [run?.provider,run?.model].map(part=>(part||'').trim()).filter(Boolean).join(' · ')
}
