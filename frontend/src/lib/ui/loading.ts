// Marks an <img>'s parent with `.loading` until the image finishes loading, so the
// slot can show a spinner instead of an empty cell while scrolling. Pass the src as
// the parameter so recycled list cells restart the indicator when their src changes.
export function loading(img: HTMLImageElement, _src?: string) {
  const slot = img.parentElement;
  const done = () => slot?.classList.remove("loading");
  const start = () => slot?.classList.toggle("loading", !img.complete);
  img.addEventListener("load", done);
  img.addEventListener("error", done);
  start();
  return {
    update: start,
    destroy() {
      img.removeEventListener("load", done);
      img.removeEventListener("error", done);
      done();
    },
  };
}
