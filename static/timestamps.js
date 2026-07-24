function parseTimestamp(ts) {
  const parts = ts.split(":").map(Number).reverse();
  let seconds = 0;
  if (parts[0]) seconds += parts[0];          // seconds
  if (parts[1]) seconds += parts[1] * 60;     // minutes
  if (parts[2]) seconds += parts[2] * 3600;   // hours
  return seconds;
}

function makeTimestampsClickable(containerId, videoId, imageId) {
  const container = document.getElementById(containerId);
  const video = document.getElementById(videoId);
  const image = document.getElementById(imageId);
  const videoContainer = document.getElementById('videoContainer');
  const imageContainer = document.getElementById('imageContainer');

  // Regex for timestamps: [h:mm:ss] or [mm:ss] or [ss]
  const timestampRegex = /\[(\d{1,2}(?::\d{2}){0,2})\]/g;
  // Regex for rotations: [rotate90], [rotate180], [rotate270], [rotate0]
  const rotateRegex = /\[rotate(0|90|180|270)\]/g;
  // Regex for URLs: http(s):// or www. links
  const urlRegex = /\b((?:https?:\/\/|www\.)[^\s<]+)/gi;
  // Regex for markdown-style links: [link text](https://example.com)
  const markdownLinkRegex = /\[([^\[\]]+)\]\(((?:https?:\/\/|www\.)[^)\s]+)\)/gi;

  // Pull out markdown links first and swap in placeholders, so the raw URL
  // inside them doesn't get double-processed by the bare-URL regex below.
  const mdLinks = [];
  let html = container.innerHTML.replace(markdownLinkRegex, (match, text, url) => {
    const href = url.startsWith("http") ? url : `https://${url}`;
    const placeholder = `\u0000MDLINK${mdLinks.length}\u0000`;
    mdLinks.push(`<a href="${href}" class="external-link" target="_blank" rel="noopener noreferrer">${text}</a>`);
    return placeholder;
  });

  // Replace bare URLs, then timestamps, then rotations
  html = html
    .replace(urlRegex, (match) => {
      // Strip common trailing punctuation that isn't part of the URL
      const trailingPunct = /[.,;:!?)\]"'>]+$/;
      const trimmed = match.match(trailingPunct) ? match.replace(trailingPunct, "") : match;
      const suffix = match.slice(trimmed.length);
      const href = trimmed.startsWith("http") ? trimmed : `https://${trimmed}`;
      return `<a href="${href}" class="external-link" target="_blank" rel="noopener noreferrer">${trimmed}</a>${suffix}`;
    })
    .replace(timestampRegex, (match, ts) => {
      const seconds = parseTimestamp(ts);
      return `<a href="#" class="timestamp" data-time="${seconds}">${match}</a>`;
    })
    .replace(rotateRegex, (match, angle) => {
      return `<a href="#" class="rotate" data-angle="${angle}">${match}</a>`;
    });

  // Restore the markdown links now that no other regex can touch them
  container.innerHTML = html.replace(/\u0000MDLINK(\d+)\u0000/g, (_, i) => mdLinks[Number(i)]);
  // Handle clicks
  container.addEventListener("click", e => {
    if (e.target.classList.contains("timestamp")) {
      e.preventDefault();
      const time = Number(e.target.dataset.time);
      if (video) {
        video.currentTime = time;
        video.play();
        window.scrollTo({ top: 0, behavior: "smooth" });
      }
    } else if (e.target.classList.contains("rotate")) {
      e.preventDefault();
      const angle = Number(e.target.dataset.angle);

      if (video) {
		applyRotation(video, angle);
	  } else if (image) {
		applyRotation(image, angle);
	  }
	}
  });
}

function applyRotation(element, angle) {
  element.style.transform = `rotate(${angle}deg)`;
  element.style.transformOrigin = "center center";
}

// Run it
document.addEventListener("DOMContentLoaded", () => {
  makeTimestampsClickable("current-description", "videoPlayer", "imageViewer");
});