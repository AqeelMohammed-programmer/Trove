// Applies the saved colour theme before the page paints, so there is no flash
// of the wrong theme. Loaded synchronously from the document head. A missing or
// unreadable preference falls back to the system setting (no attribute set).
(function () {
  "use strict";
  try {
    var choice = window.localStorage.getItem("trove-theme");
    if (choice === "light" || choice === "dark") {
      document.documentElement.setAttribute("data-theme", choice);
    }
  } catch (error) {
    /* storage disabled: keep the system default */
  }
})();
