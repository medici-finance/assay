// Ask Assay pane keyboard path (§7.6): `/` focuses the question box, Enter
// asks (the form's own default), Esc clears it.
//
// This file deliberately does nothing else. It does not fetch, it does not
// render, and it never writes model text into the DOM — every string on this
// pane is produced server-side by html/template, which escapes it. A client
// that built markup out of an answer would defeat the render control the
// template layer exists to hold.
(function () {
  "use strict";
  var input = document.getElementById("q");
  if (!input) {
    return;
  }
  document.addEventListener("keydown", function (e) {
    if (e.defaultPrevented || e.ctrlKey || e.metaKey || e.altKey) {
      return;
    }
    var typing = document.activeElement === input;
    if (e.key === "/" && !typing) {
      e.preventDefault();
      input.focus();
      input.select();
      return;
    }
    if (e.key === "Escape" && typing) {
      e.preventDefault();
      input.value = "";
    }
  });
})();
