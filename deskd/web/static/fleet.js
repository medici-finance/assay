"use strict";
// Spatial fleet map client (the service contract, one-week experiment).
//
// This file is READ-ONLY BY CONSTRUCTION: it contains no fetch, no XHR, no
// WebSocket, and stages nothing — the two verbs it uses are CSS transforms
// (pan/zoom) on the canvas element already in the DOM, and a plain page
// reload (a normal GET navigation, not a mutation) for auto-refresh. It is
// deliberately a SEPARATE file from app.js, whose postForm() is the write
// path for the answer flow this page must never share code with
// — a reviewer auditing "does /fleet write anything" can read this whole
// file without also auditing app.js's write helpers.

(function () {
  var AUTO_REFRESH_MS = 30000; // auto-refresh (brief Task 1) — a plain reload, not a background fetch

  var viewport = document.querySelector(".fleet-viewport");
  var canvas = document.getElementById("fleet-canvas");
  if (!viewport || !canvas) return; // could-not-check state has neither element; nothing to wire up

  var scale = 1, tx = 0, ty = 0;
  var dragging = false, lastX = 0, lastY = 0;

  function apply() {
    canvas.style.transform = "translate(" + tx + "px, " + ty + "px) scale(" + scale + ")";
  }

  function zoomBy(factor) {
    scale = Math.min(3, Math.max(0.3, scale * factor));
    apply();
  }

  viewport.addEventListener("wheel", function (e) {
    e.preventDefault();
    zoomBy(e.deltaY < 0 ? 1.1 : 0.9);
  }, { passive: false });

  viewport.addEventListener("mousedown", function (e) {
    dragging = true;
    lastX = e.clientX; lastY = e.clientY;
    viewport.classList.add("panning");
  });
  window.addEventListener("mousemove", function (e) {
    if (!dragging) return;
    tx += e.clientX - lastX;
    ty += e.clientY - lastY;
    lastX = e.clientX; lastY = e.clientY;
    apply();
  });
  window.addEventListener("mouseup", function () {
    dragging = false;
    viewport.classList.remove("panning");
  });

  var zoomIn = document.querySelector(".fleet-zoom-in");
  var zoomOut = document.querySelector(".fleet-zoom-out");
  var zoomReset = document.querySelector(".fleet-zoom-reset");
  if (zoomIn) zoomIn.addEventListener("click", function () { zoomBy(1.2); });
  if (zoomOut) zoomOut.addEventListener("click", function () { zoomBy(1 / 1.2); });
  if (zoomReset) zoomReset.addEventListener("click", function () { scale = 1; tx = 0; ty = 0; apply(); });

  setTimeout(function () { location.reload(); }, AUTO_REFRESH_MS);
})();
