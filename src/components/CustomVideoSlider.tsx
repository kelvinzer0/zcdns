import { ChevronLeft, ChevronRight, PlayCircle } from 'lucide-react';
import React, { createRef, useEffect, useRef, useState } from 'react';

interface Video {
  src: string;
  title: string;
  description: string;
  poster: string;
}

interface CustomVideoSliderProps {
  videos: Video[];
}

export const CustomVideoSlider: React.FC<CustomVideoSliderProps> = ({ videos }) => {
  const [currentIndex, setCurrentIndex] = useState(0);
  const [isPlaying, setIsPlaying] = useState(false);
  const videoRefs = useRef<React.RefObject<HTMLVideoElement>[]>([]);

  if (videoRefs.current.length !== videos.length) {
    videoRefs.current = Array(videos.length).fill(null).map((_, i) => videoRefs.current[i] || createRef<HTMLVideoElement>());
  }

  // Effect to control video playback
  useEffect(() => {
    const currentVideoRef = videoRefs.current[currentIndex];
    if (currentVideoRef && currentVideoRef.current) {
      if (isPlaying) {
        currentVideoRef.current.play().catch(error => {
          console.error("Error attempting to play video:", error);
          // If play fails, reset the state
          setIsPlaying(false);
        });
      } else {
        currentVideoRef.current.pause();
      }
    }
  }, [currentIndex, isPlaying]);

  const goToPrevious = () => {
    const isFirstSlide = currentIndex === 0;
    const newIndex = isFirstSlide ? videos.length - 1 : currentIndex - 1;
    setCurrentIndex(newIndex);
    setIsPlaying(false);
  };

  const goToNext = () => {
    const isLastSlide = currentIndex === videos.length - 1;
    const newIndex = isLastSlide ? 0 : currentIndex + 1;
    setCurrentIndex(newIndex);
    setIsPlaying(false);
  };

  const handlePlayButtonClick = () => {
    setIsPlaying(true);
  };
  
  const handleVideoEnd = () => {
    setIsPlaying(false);
  };

  return (
    <div className="relative w-full max-w-4xl mx-auto">
      <div className="relative h-0 pb-[56.25%]">
        {videos.map((video, index) => (
          <div
            key={index}
            className={`absolute top-0 left-0 w-full h-full transition-opacity duration-500 ${
              index === currentIndex ? 'opacity-100' : 'opacity-0 pointer-events-none'
            }`}
          >
            <video
              ref={videoRefs.current[index]}
              src={video.src}
              poster={video.poster}
              className="w-full h-full"
              controls={isPlaying}
              onEnded={handleVideoEnd}
              playsInline
            />
            {!isPlaying && (
              <div className="absolute inset-0 flex items-center justify-center bg-black/30 bg-opacity-50">
                <button
                  className="w-20 h-20 bg-white/20 hover:bg-white/30 flex items-center justify-center"
                  onClick={handlePlayButtonClick}
                >
                  <PlayCircle className="h-16 w-16 text-white" />
                </button>
              </div>
            )}
          </div>
        ))}
      </div>
      <button
        onClick={goToPrevious}
        className="absolute top-1/2 left-4 -translate-y-1/2 bg-black/50 text-white p-2"
      >
        <ChevronLeft className="h-6 w-6" />
      </button>
      <button
        onClick={goToNext}
        className="absolute top-1/2 right-4 -translate-y-1/2 bg-black/50 text-white p-2"
      >
        <ChevronRight className="h-6 w-6" />
      </button>
    </div>
  );
};
