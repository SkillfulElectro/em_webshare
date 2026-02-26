document.getElementById("chooseBtn").addEventListener("click", function () {
  const selectionType = document.querySelector(
    'input[name="selectionType"]:checked'
  ).value;
  if (selectionType === "file") {
    document.getElementById("fileInputFile").click();
  } else {
    document.getElementById("fileInputFolder").click();
  }
});

const handleUpload = (files, isDirectory) => {
  if (!files.length) return;

  const totalSize = files.reduce((acc, file) => acc + file.size, 0);
  let uploadedSize = 0;

  document.getElementById("progressContainer").style.display = "block";
  updateProgress(0);

  const uploadFile = (file) => {
    return new Promise((resolve, reject) => {
      const formData = new FormData();
      formData.append("file", file);
      if (isDirectory) {
        formData.append("relativePath", file.webkitRelativePath);
      }

      const xhr = new XMLHttpRequest();
      xhr.open("POST", "/upload", true);

      xhr.upload.onprogress = function (event) {
        if (event.lengthComputable) {
          const overallProgress =
            ((uploadedSize + event.loaded) / totalSize) * 100;
          updateProgress(overallProgress);
        }
      };

      xhr.onload = function () {
        if (xhr.status === 200) {
          uploadedSize += file.size;
          resolve();
        } else {
          reject(`Error uploading file: ${file.name} - ${xhr.status}`);
        }
      };

      xhr.onerror = function () {
        reject(`Error uploading file: ${file.name}`);
      };

      xhr.send(formData);
    });
  };

  const uploadFiles = async () => {
    for (const file of files) {
      try {
        await uploadFile(file);
        console.log(`Uploaded: ${file.name}`);
      } catch (error) {
        console.error(error);
        alert(error);
      }
    }

    alert("All files uploaded successfully!");
    document.getElementById("progressContainer").style.display = "none";
  };

  uploadFiles();
};

document
  .getElementById("fileInputFile")
  .addEventListener("change", function () {
    const files = Array.from(this.files);
    handleUpload(files, false);
  });

document
  .getElementById("fileInputFolder")
  .addEventListener("change", function () {
    const files = Array.from(this.files);
    handleUpload(files, true);
  });

function updateProgress(percent) {
  document.getElementById("progressBarInner").style.width = percent + "%";
  document.getElementById("progressText").textContent =
    Math.round(percent) + "%";
}

document.getElementById("downloadBtn").addEventListener("click", function () {
  fetch("/check-file")
    .then((response) => response.json())
    .then((data) => {
      if (data.fileAvailable) {
        window.location.href = "/download";
      } else {
        alert("No file available for download.");
      }
    })
    .catch((error) => {
      alert("Error checking for file: " + error);
    });
});
