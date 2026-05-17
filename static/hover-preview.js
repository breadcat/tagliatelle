
document.addEventListener('DOMContentLoaded', function () {
  const IMAGE_EXTENSIONS = /\.(jpe?g|png|gif|webp)$/i;
  const VIDEO_EXTENSIONS = /\.(mp4|webm|m4v)$/i;

  const tooltip = document.createElement('div');
  tooltip.id = 'thumb-tooltip';
  Object.assign(tooltip.style, {
    position: 'fixed',
    display: 'none',
    pointerEvents: 'none',
    zIndex: '9999',
    border: '1px solid #888',
    background: '#fff',
    padding: '3px',
    borderRadius: '4px',
    boxShadow: '0 2px 8px rgba(0,0,0,0.3)',
  });
  const tooltipImg = document.createElement('img');
  tooltipImg.style.maxWidth = '200px';
  tooltipImg.style.maxHeight = '200px';
  tooltipImg.style.display = 'block';
  tooltip.appendChild(tooltipImg);
  document.body.appendChild(tooltip);

  document.querySelectorAll('.file-item a').forEach(function (link) {
    const filename = link.title || link.textContent.trim();
    const isImage = IMAGE_EXTENSIONS.test(filename);
    const isVideo = VIDEO_EXTENSIONS.test(filename);
    if (!isImage && !isVideo) return;

    const thumbUrl = isVideo
      ? '/uploads/thumbnails/' + filename + '.jpg'
      : '/uploads/' + filename;

    link.addEventListener('mouseenter', function () {
      tooltipImg.src = thumbUrl;
      tooltip.style.display = 'block';
    });

    link.addEventListener('mousemove', function (e) {
      const pad = 16;
      const tw = tooltip.offsetWidth;
      const th = tooltip.offsetHeight;
      let x = e.clientX + pad;
      let y = e.clientY + pad;
      if (x + tw > window.innerWidth)  x = e.clientX - tw - pad;
      if (y + th > window.innerHeight) y = e.clientY - th - pad;
      tooltip.style.left = x + 'px';
      tooltip.style.top  = y + 'px';
    });

    link.addEventListener('mouseleave', function () {
      tooltip.style.display = 'none';
      tooltipImg.src = '';
    });
  });
});
