// ChatPort preview page script. v0.1 only fills in the running version;
// the interactive parse/select/download flow ships in v0.2 (see README
// roadmap). Everything stays local: the page talks only to this loopback
// server.
(function () {
  var el = document.getElementById("version");
  if (!el) return;
  fetch("/api/version")
    .then(function (r) {
      if (!r.ok) throw new Error("http " + r.status);
      return r.json();
    })
    .then(function (v) {
      el.textContent = "v" + (v.version || "dev");
    })
    .catch(function () {
      el.textContent = "v0.1.0";
    });
})();
