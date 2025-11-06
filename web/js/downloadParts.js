function downloadParts(doc, ids) {
  // Create a temporary form and populate it with the resource ids
  const form = doc.createElement("form");
  form.method = "POST";
  form.action = "/resources/parts";
  form.style.display = "none";

  // Add hidden input with IDs
  ids.forEach((id) => {
    const input = doc.createElement("input");
    input.type = "hidden";
    input.name = "resourceId"; // same name for all
    input.value = id;
    form.appendChild(input);
  });

  // Append, submit, then remove
  doc.body.appendChild(form);
  form.submit();
  doc.body.removeChild(form);
}
