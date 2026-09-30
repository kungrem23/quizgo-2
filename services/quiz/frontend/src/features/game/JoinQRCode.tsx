import { useEffect, useState } from 'react';
import QRCode from 'qrcode';

export function JoinQRCode({ value }: { value: string }) {
  const [source, setSource] = useState('');

  useEffect(() => {
    let active = true;
    setSource('');
    void QRCode.toDataURL(value, {
      width: 240,
      margin: 1,
      errorCorrectionLevel: 'M',
      color: { dark: '#211B3D', light: '#FFFFFF' },
    })
      .then((result) => {
        if (active) setSource(result);
      })
      .catch(() => {
        if (active) setSource('');
      });
    return () => {
      active = false;
    };
  }, [value]);

  return source ? (
    <img className="host-join-qr" src={source} alt="QR-код ссылки для подключения к игре" />
  ) : (
    <span className="host-join-qr host-join-qr-loading" aria-label="Создаём QR-код" />
  );
}
