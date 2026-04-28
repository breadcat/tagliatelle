(() => {
  // filter input HTML
  const input = document.createElement("input");
  input.type = "search";
  input.id = "list-filter";
  input.placeholder = "Filter...";
  input.autocomplete = "off";
  input.className = "list-filter-input";
  const firstDetails = document.querySelector("details");
  if (!firstDetails) return;
  firstDetails.parentNode.insertBefore(input, firstDetails);

  // filter logic
  function filter(query) {
    const q = query.trim().toLowerCase();
    const allDetails = document.querySelectorAll("details");

    allDetails.forEach((details) => {
      const summary = details.querySelector("summary");
      const items = details.querySelectorAll("li");
      const categoryText = summary ? summary.textContent.toLowerCase() : "";

      if (!q) {
        // reset
        details.removeAttribute("open");
        details.style.display = "";
        items.forEach((li) => (li.style.display = ""));
        return;
      }

      const categoryMatches = categoryText.includes(q);

      let anyItemVisible = false;
      items.forEach((li) => {
        const matches = categoryMatches || li.textContent.toLowerCase().includes(q);
        li.style.display = matches ? "" : "none";
        if (matches) anyItemVisible = true;
      });

      const visible = categoryMatches || anyItemVisible;
      details.style.display = visible ? "" : "none";

      // open on match
      if (visible) {
        details.setAttribute("open", "");
      } else {
        details.removeAttribute("open");
      }
    });
  }

  // debounce timer
  let debounceTimer;
  input.addEventListener("input", () => {
    clearTimeout(debounceTimer);
    debounceTimer = setTimeout(() => filter(input.value), 150);
  });

  // escape to clear
  input.addEventListener("keydown", (e) => {
    if (e.key === "Escape") {
      input.value = "";
      filter("");
    }
  });
})();
