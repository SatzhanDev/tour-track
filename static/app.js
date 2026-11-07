// Live search for the home page: the cards are filtered while the user
// types, without reloading the page. The browser sends the filters to
// GET /api/artists and rebuilds the list from the JSON answer.
// Without JavaScript the form still works as a normal GET form.
(() => {
  const form = document.getElementById("search");
  const list = document.getElementById("cards");
  const status = document.getElementById("status");
  const empty = document.getElementById("empty");
  if (!form || !list || !status) return;

  const DELAY_MS = 250;
  let timer;       // debounce timer
  let controller;  // aborts the previous request

  // Wait until the user stops typing before asking the server.
  form.addEventListener("input", () => {
    clearTimeout(timer);
    timer = setTimeout(search, DELAY_MS);
  });

  // Enter or the button: search now, without a page reload.
  form.addEventListener("submit", (event) => {
    event.preventDefault();
    clearTimeout(timer);
    search();
  });

  async function search() {
    const params = new URLSearchParams();
    for (const [name, value] of new FormData(form)) {
      if (value.trim() !== "") params.set(name, value.trim());
    }
    const qs = params.toString();

    // Keep the filters in the address bar, so reload and sharing work.
    history.replaceState(null, "", qs ? "/?" + qs : "/");

    // Only the latest request matters.
    controller?.abort();
    controller = new AbortController();

    try {
      const res = await fetch("/api/artists?" + qs, { signal: controller.signal });
      const data = await res.json().catch(() => ({}));
      if (!res.ok) throw new Error(data.error || "HTTP " + res.status);
      render(data);
    } catch (err) {
      if (err.name === "AbortError") return;
      status.textContent = "Search failed: " + err.message;
    }
  }

  function render(data) {
    list.replaceChildren(...data.artists.map(card));
    status.textContent = `Showing ${data.artists.length} of ${data.total} artists`;
    if (empty) empty.hidden = data.artists.length > 0;
  }

  // card builds the same markup as index.html. textContent is used instead
  // of innerHTML, so data from the API can never be run as HTML.
  function card(artist) {
    const img = el("img");
    img.src = artist.image;
    img.alt = artist.name;
    img.width = 300;
    img.height = 300;
    img.loading = "lazy";

    const body = el("div", "card-body");
    body.append(
      el("h2", "", artist.name),
      el("p", "meta", `Since ${artist.creationDate} · ${artist.concertCount} concerts`),
      el("span", "badge badge-" + artist.label.toLowerCase(), artist.label),
    );
    if (artist.risingStar) {
      body.append(" ", el("span", "badge badge-rising", "Rising Star"));
    }

    const link = el("a");
    link.href = "/artist/" + artist.id;
    link.append(img, body);

    const item = el("li", "card");
    item.append(link);
    return item;
  }

  function el(tag, className = "", text = "") {
    const node = document.createElement(tag);
    if (className) node.className = className;
    if (text) node.textContent = text;
    return node;
  }
})();
