import { useEffect, useRef, useState, type FormEvent } from 'react';
import { Link, useNavigate, useParams } from 'react-router';
import { GameSocket } from '../features/game/socket';
import { saveGameCredentials } from '../features/game/credentials';
import type { GameErrorCode } from '../features/game/types';
import { Brand, Button, ErrorBox } from '../shared/ui/ui';

function normalizeCode(value: string): string {
  return value.toUpperCase().replace(/\s/g, '').slice(0, 6);
}

function joinError(code: GameErrorCode): string {
  switch (code) {
    case 'game_not_found':
      return 'Игра с таким кодом не найдена.';
    case 'nickname_taken':
      return 'Этот никнейм уже занят в комнате.';
    case 'invalid_phase':
      return 'Игра уже началась, присоединиться к ней нельзя.';
    case 'invalid_payload':
      return 'Проверьте код и никнейм.';
    default:
      return 'Не удалось присоединиться к игре.';
  }
}

export function JoinPage() {
  const params = useParams();
  const navigate = useNavigate();
  const [code, setCode] = useState(() => normalizeCode(params.code || ''));
  const [nickname, setNickname] = useState('');
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const socketRef = useRef<GameSocket | null>(null);

  useEffect(
    () => () => {
      socketRef.current?.disconnect();
    },
    [],
  );

  async function submit(event: FormEvent) {
    event.preventDefault();
    const normalizedCode = normalizeCode(code);
    const normalizedNickname = nickname.trim();
    if (normalizedCode.length !== 6 || !normalizedNickname || normalizedNickname.length > 30) {
      setError('Введите шестизначный код и никнейм длиной до 30 символов.');
      return;
    }
    setBusy(true);
    setError('');
    socketRef.current?.disconnect();
    const socket = new GameSocket({
      onMessage: (message) => {
        if (message.type === 'joined') {
          const saved = saveGameCredentials({
            role: 'player',
            gameId: message.payload.game_id,
            code: normalizedCode,
            playerId: message.payload.player_id,
            nickname: message.payload.nickname,
            playerTicket: message.payload.ticket,
          });
          if (!saved) {
            setBusy(false);
            setError('Браузер не разрешил сохранить данные игры. Проверьте настройки хранилища.');
            socket.disconnect();
            return;
          }
          socket.disconnect();
          void navigate(`/games/${encodeURIComponent(message.payload.game_id)}/player`, {
            replace: true,
          });
        } else if (message.type === 'error') {
          setBusy(false);
          setError(joinError(message.payload.code));
        }
      },
      onProtocolError: () => {
        setBusy(false);
        setError('Сервер прислал некорректный ответ.');
      },
    });
    socketRef.current = socket;
    try {
      await socket.connect({ code: normalizedCode });
      socket.join(normalizedCode, normalizedNickname);
    } catch {
      setBusy(false);
      setError('Не удалось связаться с сервером игры.');
    }
  }

  return (
    <main className="game-foundation-page">
      <div className="game-foundation-card">
        <Brand />
        <div>
          <h1>Присоединиться к игре</h1>
          <p>Введите код с экрана ведущего и никнейм.</p>
        </div>
        {error && <ErrorBox message={error} />}
        <form className="game-foundation-form" onSubmit={(event) => void submit(event)}>
          <label>
            Код игры
            <input
              value={code}
              onChange={(event) => setCode(normalizeCode(event.target.value))}
              autoCapitalize="characters"
              autoComplete="off"
              maxLength={6}
              placeholder="ABC123"
              disabled={busy}
            />
          </label>
          <label>
            Никнейм
            <input
              value={nickname}
              onChange={(event) => setNickname(event.target.value)}
              autoComplete="nickname"
              maxLength={30}
              placeholder="Как вас зовут?"
              disabled={busy}
            />
          </label>
          <Button type="submit" busy={busy}>
            Войти в игру
          </Button>
        </form>
        <Link to="/quizzes">Вернуться к квизам</Link>
      </div>
    </main>
  );
}
