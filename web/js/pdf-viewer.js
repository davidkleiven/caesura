pdfjsLib.GlobalWorkerOptions.workerSrc = 'https://cdnjs.cloudflare.com/ajax/libs/pdf.js/3.11.174/pdf.worker.min.js';
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
        } catch(err) {
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

        pdfjsLib.getDocument(typedarray).promise.then((pdf) => {
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
        const color = assignementColor(assignmentId);
        assignmentSection.innerHTML += `
            <div id="${assignmentId}-group" class="relative group inline-block pr-2">
                <button id=${assignmentId} onclick="deleteOrJump(this)" class="flex text-white ${color} py-2 px-4 rounded-lg">
                    <span id="${assignmentId}-from">${currentPage}</span> -
                    <span id="${assignmentId}-to">${currentPage}</span>
                </button>
                <div class="absolute bottom-full left-1/2 -translate-x-1/2 mb-2
                hidden group-hover:block bg-gray-800 text-white text-xs
                rounded px-2 py-1 z-10 whitespace-nowrap">
                ${assignmentDesc}
            </div>
        `;
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

function assignementColor(assignmentId) {
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
