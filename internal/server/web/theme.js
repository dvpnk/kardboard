// Applies the chosen theme before the page is painted, so a dark page does
// not flash white first. It runs as a plain script in the head; app.js keeps
// the attribute in step afterwards. Without a choice, the system decides.
(function () {
  try {
    var theme = JSON.parse(localStorage.getItem("kb.theme"));
    if (theme === "light" || theme === "dark") document.documentElement.dataset.theme = theme;
  } catch (e) { /* no stored choice */ }
})();
