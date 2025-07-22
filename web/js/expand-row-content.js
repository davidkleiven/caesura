// htmx-expand-toggle.js

async function toggleRowContent(id) {
    const row = document.getElementById('expand-' + id);
    if (!row.classList.contains('hidden')) {
        row.classList.add('hidden');
    } else {
        const resp = await fetch(`/resources/${id}/content`)
        if (!resp.ok) {
            console.log("An error occured during fetch")
        }

        row.innerHTML = await resp.text()
        row.classList.remove('hidden')
    }
  }
