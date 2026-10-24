// Trove progressive enhancement. Everything here is optional: every action
// also works without JavaScript. Kept as a separate file so the strict CSP can
// forbid inline scripts.
(function () {
  "use strict";

  // Copy-to-clipboard for link attachments (A-7). The button carries the URL
  // in data-copy; without JavaScript the URL is visible as text.
  document.addEventListener("click", function (event) {
    var button = event.target.closest("[data-copy]");
    if (!button) {
      return;
    }
    event.preventDefault();
    var value = button.getAttribute("data-copy");
    if (!value || !navigator.clipboard) {
      return;
    }
    navigator.clipboard.writeText(value).then(function () {
      var original = button.textContent;
      button.textContent = "Copied";
      window.setTimeout(function () {
        button.textContent = original;
      }, 1500);
    });
  });
})();
