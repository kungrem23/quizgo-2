import { useState } from 'react';
import { useQuery } from '@tanstack/react-query';
import { api } from '../api/client';

type Props = { imageId: string; url?: string; alt: string; className?: string };

// A changed image or URL resets the one-time refresh after an expired S3 link.
export function QuizImage(props: Props) {
  return <ImageWithRefresh key={`${props.imageId}:${props.url || ''}`} {...props} />;
}

function ImageWithRefresh({ imageId, url, alt, className }: Props) {
  const [refresh, setRefresh] = useState(!url);
  const [failed, setFailed] = useState(false);
  const link = useQuery({
    queryKey: ['image-url', imageId],
    queryFn: ({ signal }) => api.imageURL(imageId, signal),
    enabled: refresh,
    staleTime: 0,
    gcTime: 0,
    retry: 1,
    refetchOnWindowFocus: false,
  });
  const source = refresh ? link.data?.url : url;
  if (failed || (refresh && link.isError)) {
    return (
      <div className="image-placeholder" role="status">
        Не удалось загрузить изображение.
        <button
          type="button"
          className="text-button"
          onClick={() => {
            setFailed(false);
            setRefresh(true);
            void link.refetch();
          }}
        >
          Повторить
        </button>
      </div>
    );
  }
  if (!source || (refresh && link.isFetching)) {
    return (
      <div className="image-placeholder" role="status">
        Загрузка изображения…
      </div>
    );
  }
  return (
    <img
      className={className}
      src={source}
      alt={alt}
      referrerPolicy="no-referrer"
      onError={() => (refresh ? setFailed(true) : setRefresh(true))}
    />
  );
}
