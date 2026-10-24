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

  // --- tag autocomplete (T-2) --------------------------------------------
  // Tags live in one comma-separated input. A native <datalist> matches the
  // whole input value, so it can only ever suggest the first tag; this widget
  // suggests for the token at the caret instead. It asks the server for the
  // matches, so the "existing tags, prefix match, active only" rule stays in
  // one place, and it replaces only the token being typed. Without JavaScript
  // the field stays a plain input and the datalist remains as a fallback.
  function enhanceTagAutocomplete(input) {
    var endpoint = input.getAttribute("data-tag-suggest");
    if (!endpoint) {
      return;
    }

    input.removeAttribute("list"); // the custom listbox replaces the datalist
    input.setAttribute("role", "combobox");
    input.setAttribute("aria-autocomplete", "list");
    input.setAttribute("aria-haspopup", "listbox");
    input.setAttribute("aria-expanded", "false");

    var list = document.createElement("ul");
    list.className = "tag-suggestions";
    list.id = input.id + "-suggestions";
    list.setAttribute("role", "listbox");
    list.hidden = true;
    input.setAttribute("aria-controls", list.id);
    input.parentNode.appendChild(list);

    var names = [];
    var active = -1;
    var request = null;
    var debounce = 0;

    function close() {
      list.hidden = true;
      list.textContent = "";
      names = [];
      active = -1;
      input.setAttribute("aria-expanded", "false");
      input.removeAttribute("aria-activedescendant");
    }

    // token returns the comma-delimited tag around the caret and its bounds.
    function token() {
      var pos = input.selectionStart;
      if (pos === null || pos === undefined) {
        pos = input.value.length;
      }
      var before = input.value.slice(0, pos);
      var start = before.lastIndexOf(",") + 1;
      var nextComma = input.value.indexOf(",", pos);
      var end = nextComma === -1 ? input.value.length : nextComma;
      return { start: start, end: end, text: input.value.slice(start, end) };
    }

    // taken lists committed names, excluding the token being typed.
    function taken(current) {
      var others = input.value.slice(0, current.start) + "," + input.value.slice(current.end);
      return others.split(",").map(function (part) {
        return part.trim().toLowerCase();
      });
    }

    function open(items) {
      list.textContent = "";
      names = items;
      active = -1;
      items.forEach(function (name, index) {
        var option = document.createElement("li");
        option.className = "tag-suggestion";
        option.id = list.id + "-" + index;
        option.setAttribute("role", "option");
        option.setAttribute("aria-selected", "false");
        option.textContent = name;
        option.addEventListener("mousedown", function (event) {
          // mousedown, not click: the input keeps focus and no blur fires.
          event.preventDefault();
          accept(index);
        });
        list.appendChild(option);
      });
      list.hidden = items.length === 0;
      input.setAttribute("aria-expanded", String(items.length > 0));
    }

    function highlight(index) {
      if (names.length === 0) {
        return;
      }
      active = (index + names.length) % names.length;
      Array.prototype.forEach.call(list.children, function (option, i) {
        var selected = i === active;
        option.setAttribute("aria-selected", String(selected));
        option.classList.toggle("is-active", selected);
      });
      input.setAttribute("aria-activedescendant", list.id + "-" + active);
    }

    function accept(index) {
      var name = names[index];
      if (name === undefined) {
        return;
      }
      var current = token();
      var head = input.value.slice(0, current.start).replace(/[\s,]+$/, "");
      var tail = input.value.slice(current.end).replace(/^[\s,]+/, "");
      var prefix = (head === "" ? "" : head + ", ") + name;
      input.value = tail === "" ? prefix + ", " : prefix + ", " + tail;
      var caret = prefix.length + 2;
      input.focus();
      input.setSelectionRange(caret, caret);
      close();
    }

    function load(query) {
      if (request) {
        request.abort();
      }
      request = new AbortController();
      window
        .fetch(endpoint + "?q=" + encodeURIComponent(query), {
          headers: { Accept: "application/json" },
          signal: request.signal,
        })
        .then(function (response) {
          if (!response.ok) {
            throw new Error("tag suggestions unavailable");
          }
          return response.json();
        })
        .then(function (data) {
          // A slower response must not overwrite suggestions for newer text.
          if (token().text.trim() !== query) {
            return;
          }
          var seen = taken(token());
          open(
            (data.tags || []).filter(function (name) {
              return seen.indexOf(name.toLowerCase()) === -1;
            })
          );
        })
        .catch(function () {
          // Offline, aborted or malformed: leave the field as plain text.
        });
    }

    function schedule() {
      window.clearTimeout(debounce);
      var query = token().text.trim();
      if (query === "") {
        close();
        return;
      }
      debounce = window.setTimeout(function () {
        load(query);
      }, 120);
    }

    input.addEventListener("input", schedule);
    input.addEventListener("keydown", function (event) {
      if (list.hidden) {
        return;
      }
      if (event.key === "ArrowDown") {
        event.preventDefault();
        highlight(active + 1);
      } else if (event.key === "ArrowUp") {
        event.preventDefault();
        highlight(active - 1);
      } else if (event.key === "Enter" && active >= 0) {
        event.preventDefault();
        accept(active);
      } else if (event.key === "Escape") {
        close();
      }
    });
    input.addEventListener("blur", function () {
      window.setTimeout(close, 0);
    });
  }

  document.querySelectorAll("input[data-tag-suggest]").forEach(enhanceTagAutocomplete);
})();
