### Fixed
- The inbox now labels and derives a ruling only when exactly one visible option carries the standalone `(recommended)` marker. Ordinary recommendation wording, negated markers, hidden comment text, and missing or ambiguous markers cannot create a recommendation.
- The marker may be followed by `.`, `,`, `;`, `:`, `!` or `?`, so `(recommended).` closing a bold lead sentence still counts. The shell inbox, `deskinbox` and the human-decision record now share one marker grammar: adjacent markers count as two, and only a space or tab is a boundary.
