// gopatina.org was one page whose sections had #anchors; they are now pages
// of their own. A server never sees the part after #, so this sends an old
// link such as https://gopatina.org/#faq to its new page. It runs on the
// home page only and does nothing for any other address.
(function () {
  var moved = {
    "#subset": "/how-it-works#subset",
    "#how": "/how-it-works",
    "#try": "/get-started",
    "#roadmap": "/roadmap",
    "#quality": "/correctness",
    "#evidence": "/evidence",
    "#benchmarks": "/benchmarks",
    "#faq": "/faq"
  };
  function follow() {
    var to = moved[window.location.hash];
    if (to) {
      window.location.replace(to);
    }
  }
  follow();
  window.addEventListener("hashchange", follow);
})();
