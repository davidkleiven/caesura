const fileInput = document.getElementById('file-input');
const canvas = document.getElementById('pdf-canvas');
const ctx = canvas.getContext('2d');

const pageNumSpan = document.getElementById('page-num');
const pageCountSpan = document.getElementById('page-count');
const prevPageBtn = document.getElementById('prev-page');
const nextPageBtn = document.getElementById('next-page');

let pdfDoc = null;
let currentPage = 1;

const renderPage = (num) => {
    pdfDoc.getPage(num).then((page) => {
        const viewport = page.getViewport({ scale: 1.5 });
        canvas.height = viewport.height;
        canvas.width = viewport.width;

        const renderContext = {
        canvasContext: ctx,
        viewport: viewport,
        };
        page.render(renderContext);
        pageNumSpan.textContent = num;
    });
    };

    const queueRenderPage = (num) => {
    if (num < 1 || num > pdfDoc.numPages) return;
    currentPage = num;
    renderPage(currentPage);
    };

    prevPageBtn.addEventListener('click', () => {
    queueRenderPage(currentPage - 1);
    });

    nextPageBtn.addEventListener('click', () => {
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
