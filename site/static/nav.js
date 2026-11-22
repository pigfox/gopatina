// The site menu on narrow screens: the button shows and hides the
// navigation, Escape closes it and returns focus to the button. Without
// JavaScript, nojs.css hides the button and always shows the navigation.
(function () {
  var button = document.querySelector(".menu-button");
  var nav = document.getElementById("site-nav");
  if (!button || !nav) {
    return;
  }
  function set(open) {
    button.setAttribute("aria-expanded", open ? "true" : "false");
    nav.classList.toggle("open", open);
  }
  button.addEventListener("click", function () {
    set(button.getAttribute("aria-expanded") !== "true");
  });
  document.addEventListener("keydown", function (e) {
    if (e.key === "Escape" && button.getAttribute("aria-expanded") === "true") {
      set(false);
      button.focus();
    }
  });
})();
