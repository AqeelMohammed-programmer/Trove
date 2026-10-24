// Trove progressive enhancement. Everything here is optional: every action also
// works without JavaScript. Kept as a separate file so the strict CSP can
// forbid inline scripts.
(function () {
  "use strict";

  // --- colour theme preference -------------------------------------------
  // theme.js has already applied the saved choice before paint; here we only
  // wire the buttons and reflect the current state.
  var root = document.documentElement;
  var themeButtons = document.querySelectorAll("[data-theme-set]");

  function currentTheme() {
    try {
      var saved = window.localStorage.getItem("trove-theme");
      if (saved === "light" || saved === "dark") {
        return saved;
      }
    } catch (error) {
      /* storage disabled */
    }
    return "system";
  }

  function reflect(value) {
    themeButtons.forEach(function (button) {
      button.setAttribute("aria-pressed", String(button.getAttribute("data-theme-set") === value));
    });
  }

  function apply(value) {
    if (value === "system") {
      root.removeAttribute("data-theme");
    } else {
      root.setAttribute("data-theme", value);
    }
    reflect(value);
  }

  if (themeButtons.length) {
    reflect(currentTheme());
    themeButtons.forEach(function (button) {
      button.addEventListener("click", function () {
        var value = button.getAttribute("data-theme-set");
        try {
          if (value === "system") {
            window.localStorage.removeItem("trove-theme");
          } else {
            window.localStorage.setItem("trove-theme", value);
          }
        } catch (error) {
          /* storage disabled: apply for this page only */
        }
        apply(value);
      });
    });
  }

  // --- copy-to-clipboard for link attachments (A-7) ----------------------
  // The button carries the URL in data-copy; without JavaScript the URL is
  // visible as text.
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
