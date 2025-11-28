document.body.addEventListener("htmx:responseError", function (event) {
  const status = event.detail.xhr.status;
  const url = event.detail.path;
  const message = event.detail.message ?? "";

  // Don't alert on BadRequest
  if (status === 400) return;

  console.error("HTMX error response:", event.detail);
  alert(`Request failed: ${status} for ${url}: ${message}`);
});
