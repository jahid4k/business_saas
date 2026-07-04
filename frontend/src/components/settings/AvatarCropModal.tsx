// src/components/settings/AvatarCropModal.tsx
"use client";

import { useCallback, useState } from "react";
import Cropper, { type Area } from "react-easy-crop";
import { Loader2, X, ZoomIn } from "lucide-react";

// Fixed output canvas size — matches the backend's own canonical resize
// target (avatarCanvasSize in internal/user/avatar.go). Sending something
// already close to that size keeps the upload fast; the backend still
// independently re-validates and re-resizes regardless.
const OUTPUT_SIZE = 512;
const JPEG_QUALITY = 0.92;

function loadImage(src: string): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const img = new Image();
    img.addEventListener("load", () => resolve(img));
    img.addEventListener("error", (e) => reject(e));
    img.src = src;
  });
}

async function extractCroppedSquare(
  imageSrc: string,
  area: Area,
): Promise<Blob> {
  const image = await loadImage(imageSrc);
  const canvas = document.createElement("canvas");
  canvas.width = OUTPUT_SIZE;
  canvas.height = OUTPUT_SIZE;
  const ctx = canvas.getContext("2d");
  if (!ctx) throw new Error("Canvas 2D context unavailable");

  ctx.drawImage(
    image,
    area.x,
    area.y,
    area.width,
    area.height,
    0,
    0,
    OUTPUT_SIZE,
    OUTPUT_SIZE,
  );

  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (blob) => (blob ? resolve(blob) : reject(new Error("toBlob failed"))),
      "image/jpeg",
      JPEG_QUALITY,
    );
  });
}

export default function AvatarCropModal({
  imageSrc,
  onConfirm,
  onCancel,
}: {
  imageSrc: string;
  onConfirm: (blob: Blob) => void | Promise<void>;
  onCancel: () => void;
}) {
  const [crop, setCrop] = useState({ x: 0, y: 0 });
  const [zoom, setZoom] = useState(1);
  const [croppedAreaPixels, setCroppedAreaPixels] = useState<Area | null>(null);
  const [processing, setProcessing] = useState(false);

  const handleCropComplete = useCallback((_area: Area, areaPixels: Area) => {
    setCroppedAreaPixels(areaPixels);
  }, []);

  const handleConfirm = async () => {
    if (!croppedAreaPixels || processing) return;
    setProcessing(true);
    try {
      const blob = await extractCroppedSquare(imageSrc, croppedAreaPixels);
      await onConfirm(blob);
    } finally {
      setProcessing(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-[100] flex items-center justify-center p-4"
      role="dialog"
      aria-modal="true"
      aria-label="Crop avatar"
    >
      {/* Backdrop */}
      <div
        className="absolute inset-0 bg-black/60"
        onClick={processing ? undefined : onCancel}
      />

      {/* Panel */}
      <div className="relative w-full max-w-md rounded-2xl border border-(--border) bg-(--bg-surface) shadow-2xl overflow-hidden">
        <div className="flex items-center justify-between px-5 py-4 border-b border-(--border)">
          <p
            className="text-sm font-semibold text-[var(--text-primary)]"
            style={{ fontFamily: "var(--font-syne, Syne, sans-serif)" }}
          >
            Crop your photo
          </p>
          <button
            onClick={onCancel}
            disabled={processing}
            className="p-1.5 rounded-md text-(--text-muted) hover:text-[var(--text-primary)] hover:bg-(--bg-elevated) transition-colors disabled:opacity-50"
          >
            <X size={16} />
          </button>
        </div>

        {/* Crop area — react-easy-crop absolutely-positions itself to fill
            this container, so it needs an explicit height and position:relative
            (via the `relative` class below). */}
        <div className="relative w-full bg-black" style={{ height: 320 }}>
          <Cropper
            image={imageSrc}
            crop={crop}
            zoom={zoom}
            aspect={1}
            cropShape="round"
            showGrid={false}
            onCropChange={setCrop}
            onZoomChange={setZoom}
            onCropComplete={handleCropComplete}
          />
        </div>

        {/* Zoom control */}
        <div className="flex items-center gap-3 px-5 py-4">
          <ZoomIn size={15} className="text-(--text-muted) flex-shrink-0" />
          <input
            type="range"
            min={1}
            max={3}
            step={0.01}
            value={zoom}
            onChange={(e) => setZoom(Number(e.target.value))}
            className="w-full accent-purple-500"
            aria-label="Zoom"
          />
        </div>

        {/* Actions */}
        <div className="flex items-center gap-3 px-5 pb-5">
          <button
            onClick={onCancel}
            disabled={processing}
            className="flex-1 py-2.5 rounded-lg text-sm font-medium text-[var(--text-secondary)] border border-(--border) hover:bg-(--bg-elevated) transition-colors disabled:opacity-50"
          >
            Cancel
          </button>
          <button
            onClick={handleConfirm}
            disabled={processing || !croppedAreaPixels}
            className="flex-1 flex items-center justify-center gap-2 py-2.5 rounded-lg text-sm font-semibold text-white bg-purple-600 hover:bg-purple-500 disabled:opacity-60 disabled:cursor-not-allowed transition-colors"
          >
            {processing ? (
              <>
                <Loader2 size={14} className="animate-spin" />
                Processing…
              </>
            ) : (
              "Use this photo"
            )}
          </button>
        </div>
      </div>
    </div>
  );
}
