const paths = {
 user: "M12 3a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M4 21v-2a8 8 0 0 1 16 0v2",
 link: "m10 13 4-4 M8 15l-2 2a4 4 0 0 1-6-6l4-4a4 4 0 0 1 6 0 M16 9l2-2a4 4 0 0 1 6 6l-4 4a4 4 0 0 1-6 0",
 send: "M12 20V4 M5 11l7-7 7 7",
 chat: "M4 4h16v12H9l-5 4z M8 8h8 M8 12h5",
  panel: "M3 4h18v16H3z M9 4v16",
  board: "M3 4h18v16H3z M9 4v16 M15 4v16",
  terminal: "m5 8 4 4-4 4 M12 16h7",
  play: "m8 4 12 8-12 8z",
  close: "m6 6 12 12 M6 18 18 6",
  plus: "M12 4v16 M4 12h16",
  folder: "M3 6h7l2 3h9v11H3z",
  settings:
    "M4 6h7 M15 6h5 M4 18h3 M11 18h9 M11 3v6 M7 15v6 M4 12h12 M20 12h.01 M16 9v6",
  sun: "M12 8a4 4 0 1 0 0 8 4 4 0 0 0 0-8 M12 2v2 M12 20v2 M2 12h2 M20 12h2 M5 5l1 1 M18 18l1 1 M5 19l1-1 M18 6l1-1",
  moon: "M20 14a8 8 0 1 1-10-10 7 7 0 0 0 10 10z",
  monitor: "M3 4h18v13H3z M12 17v4 M8 21h8",
  refresh: "M20 7v5h-5 M4 17v-5h5 M6 6a8 8 0 0 1 14 6 M18 18A8 8 0 0 1 4 12",
  search: "M10 3a7 7 0 1 0 0 14 7 7 0 0 0 0-14 M15 15l6 6",
  chevron: "m9 5 7 7-7 7",
  agent: "M5 7h14v13H5z M9 12h.01 M15 12h.01 M9 16h6 M12 3v4",
  branch: "M6 3v12a4 4 0 0 0 4 4h5 M6 8a2 2 0 1 0 0-4 2 2 0 0 0 0 4 M18 8a2 2 0 1 0 0-4 2 2 0 0 0 0 4 M18 8v5a4 4 0 0 1-4 4 M18 21a2 2 0 1 0 0-4 2 2 0 0 0 0 4",
};
export function Icon({ name }: { name: keyof typeof paths }) {
  return (
    <svg
      viewBox="0 0 24 24"
      width="18"
      height="18"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.6"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      <path d={paths[name]} />
    </svg>
  );
}
