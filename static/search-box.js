const form = document.getElementById("search-form");
const input = form.q;

let openInNewTab = false;

input.addEventListener("keydown", function (e) {
    openInNewTab = e.ctrlKey && e.key === "Enter";
});

form.addEventListener("submit", function (e) {
    e.preventDefault();

    const url = "/search/" + encodeURIComponent(input.value);

    if (openInNewTab) {
        window.open(url, "_blank");
        openInNewTab = false;
    } else {
        window.location.href = url;
    }
});