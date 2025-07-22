import { getDocument, GlobalWorkerOptions } from 'https://unpkg.com/pdfjs-dist@{{.PdfJsVersion}}/build/pdf.min.mjs';
GlobalWorkerOptions.workerSrc = 'https://unpkg.com/pdfjs-dist@{{.PdfJsVersion}}/build/pdf.worker.min.mjs';

const fileInput = document.getElementById('file-input');
const canvas = document.getElementById('pdf-canvas');
const ctx = canvas.getContext('2d');

const pageNumSpan = document.getElementById('page-num');
const pageCountSpan = document.getElementById('page-count');
const prevPageBtn = document.getElementById('prev-page');
const nextPageBtn = document.getElementById('next-page');
const assignPageBtn = document.getElementById('assign-page');
const assignmentSection = document.getElementById('assignments');
const deleteOnClickCheckBox = document.getElementById('delete-on-click');
const submitBtn = document.getElementById('submit-btn');
const composerInput = document.getElementById('composer-input');
const arrangerInput = document.getElementById('arranger-input');
const titleInput = document.getElementById('title-input');

let pdfDoc = null;
let currentPage = 1;
let currentRenderTask = null;

const renderPage = (num) => {
    pdfDoc.getPage(num).then(async (page) => {
        if (!pdfDoc) return;
        if (currentRenderTask) {
            currentRenderTask.cancel();
        }
        const viewport = page.getViewport({ scale: 1.5 });
        canvas.height = viewport.height;
        canvas.width = viewport.width;

        const renderContext = {
            canvasContext: ctx,
            viewport: viewport,
        };
        currentRenderTask = page.render(renderContext);
        pageNumSpan.textContent = num;

        try {
            await currentRenderTask.promise;
        } catch (err) {
            if (err.name === 'RenderingCancelledException') {
                console.log('Rendering was cancelled.');
            } else {
                throw err;
            }
        }
    });
};

const queueRenderPage = (num) => {
    if (!pdfDoc || num < 1 || num > pdfDoc.numPages) return;
    currentPage = num;
    renderPage(currentPage);
};

prevPageBtn.addEventListener('click', () => {
    queueRenderPage(currentPage - 1);
});

nextPageBtn.addEventListener('click', () => {
    queueRenderPage(currentPage + 1);
})

assignPageBtn.addEventListener('click', () => {
    if (addAssignment() != 0) return;
    queueRenderPage(currentPage + 1);
})

fileInput.addEventListener('change', (e) => {
    const file = e.target.files[0];
    if (file && file.type === 'application/pdf') {
        const fileReader = new FileReader();
        fileReader.onload = function () {
            const typedarray = new Uint8Array(this.result);

            getDocument(typedarray).promise.then((pdf) => {
                pdfDoc = pdf;
                pageCountSpan.textContent = pdf.numPages;
                currentPage = 1;
                renderPage(currentPage);
            });
        };
        fileReader.readAsArrayBuffer(file);
    } else {
        alert('Please upload a valid PDF file.');
    }
});

submitBtn.addEventListener('click', submitPartitions)

function deleteOrJump(elem) {
    if (deleteOnClickCheckBox.checked) {
        elem.parentElement.remove();
    } else {
        const assignmentId = elem.id;
        const fromPage = parseInt(document.getElementById(`${assignmentId}-from`).textContent);
        queueRenderPage(fromPage);
    }
}

function addAssignment() {
    const chosenInstrument = document.getElementById('chosen-instrument');
    const currentPage = pageNumSpan.textContent;
    const assignmentDesc = (chosenInstrument.textContent || "") + (document.getElementById('part-number')?.value || "");
    const assignmentId = assignmentDesc.toLowerCase().replace(/\s+/g, '');

    if (!assignmentId) {
        alert('Please select an instrument and enter a part number before assigning a page.');
        return 1;
    }

    const assignmentDiv = document.getElementById(assignmentId);
    if (!assignmentDiv) {
        const color = assignmentColor(assignmentId);
        const div = document.createElement('div');
        div.id = `${assignmentId}-group`;
        div.className = 'relative group inline-block pr-2';

        const btn = document.createElement('button');
        btn.id = assignmentId;
        btn.className = `flex text-white ${color} py-2 px-4 rounded-lg`;
        btn.innerHTML = `<span id="${assignmentId}-from">${currentPage}</span> - <span id="${assignmentId}-to">${currentPage}</span>`;
        btn.addEventListener('click', () => deleteOrJump(btn));

        const desc = document.createElement('div');
        desc.className = 'absolute bottom-full left-1/2 -translate-x-1/2 mb-2 hidden group-hover:block bg-gray-800 text-white text-xs rounded px-2 py-1 z-10 whitespace-nowrap';
        desc.textContent = assignmentDesc;

        div.appendChild(btn);
        div.appendChild(desc);

        assignmentSection.appendChild(div);
    } else {
        const currentFrom = parseInt(document.getElementById(`${assignmentId}-from`).textContent)
        const newTo = parseInt(currentPage);
        if (newTo >= currentFrom) {
            document.getElementById(`${assignmentId}-to`).textContent = newTo;
        } else {
            alert('Attempting to assign a page before the current first page. If you want to change the first page, please remove the assignment and reassign it.');
            return 1;
        }
    }

    return 0;
}

function assignmentColor(assignmentId) {
    if (assignmentId.toLowerCase().includes("trumpet") || assignmentId.toLowerCase().includes("cornet")) {
        return "bg-red-400 hover:bg-red-500";
    } else if (assignmentId.toLowerCase().includes("trombone")) {
        return "bg-yellow-400 hover:bg-yellow-500";
    }
    else if (assignmentId.toLowerCase().includes("clarinet")) {
        return "bg-green-400 hover:bg-green-500";
    }
    else if (assignmentId.toLowerCase().includes("saxophone")) {
        return "bg-green-600 hover:bg-green-700";
    }
    else if (assignmentId.toLowerCase().includes("flute")) {
        return "bg-purple-600 hover:bg-purple-700";
    }
    else if (assignmentId.toLowerCase().includes("oboe")) {
        return "bg-blue-400 hover:bg-blue-500";
    }
    else if (assignmentId.toLowerCase().includes("bassoon")) {
        return "bg-blue-600 hover:bg-blue-700";
    }
    else if (assignmentId.toLowerCase().includes("tuba") || assignmentId.toLowerCase().includes("bass")) {
        return "bg-yellow-600 hover:bg-yellow-700";
    }
    return "bg-gray-400 hover:bg-gray-500";
}

async function submitPartitions() {
    if (!fileInput.files.length) {
        return;
    }

    const metadata = getMetaData();
    if (!metadata) {
        return;
    }

    const formData = new FormData();
    formData.append('document', fileInput.files[0]);

    let assignments = [];
    for (const div of assignmentSection.children) {
        if (div.id.endsWith('-group')) {
            const assignmentId = div.id.replace('-group', '');
            const fromPage = document.getElementById(`${assignmentId}-from`).textContent;
            const toPage = document.getElementById(`${assignmentId}-to`).textContent;

            assignments.push({
                id: assignmentId,
                from: parseInt(fromPage),
                to: parseInt(toPage)
            });
        }
    }
    formData.append('assignments', JSON.stringify(assignments));
    formData.append('metadata', JSON.stringify(metadata));

    const response = await fetch("/resources", { method: 'POST', body: formData });
    if (!response.ok) {
        const errorText = await response.text();
        alert(`Error submitting partition: ${errorText}`);
    }
}

function getMetaData() {
    const data = {
        composer: composerInput.value.trim(),
        arranger: arrangerInput.value.trim(),
        title: titleInput.value.trim()
    }

    if (!data.composer && !data.arranger && !data.title) {
        alert('Please fill in at least one of title/composer/arranger');
        return null;
    }
    return data;
}
