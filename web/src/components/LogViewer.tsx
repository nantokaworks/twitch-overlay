import React, { useState, useEffect, useRef } from 'react';
import { Download, Trash2, Play, Pause } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from './ui/card';
import { Button } from './ui/button';
import { Input } from './ui/input';
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from './ui/select';
import { buildApiUrl } from '../utils/api';

interface LogEntry {
  timestamp: string;
  level: string;
  message: string;
  fields?: Record<string, any>;
}

interface LogViewerProps {
  embedded?: boolean;
}

export const LogViewer: React.FC<LogViewerProps> = ({ embedded = false }) => {
  const [logs, setLogs] = useState<LogEntry[]>([]);
  const [isStreaming, setIsStreaming] = useState(false);
  const [filter, setFilter] = useState('');
  const [levelFilter, setLevelFilter] = useState('all');
  const [autoScroll, setAutoScroll] = useState(true);
  const logsEndRef = useRef<HTMLDivElement>(null);
  const wsRef = useRef<WebSocket | null>(null);

  // 初回ログ取得
  useEffect(() => {
    fetchLogs();
    return () => {
      if (wsRef.current) {
        wsRef.current.close();
      }
    };
  }, []);

  // 自動スクロール
  useEffect(() => {
    if (autoScroll) {
      logsEndRef.current?.scrollIntoView({ behavior: 'smooth' });
    }
  }, [logs, autoScroll]);

  const fetchLogs = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/logs?limit=100'));
      if (!response.ok) throw new Error('Failed to fetch logs');
      
      const data = await response.json();
      setLogs(data.logs || []);
    } catch (error) {
      console.error('Failed to fetch logs:', error);
    }
  };

  const startStreaming = () => {
    const wsUrl = buildApiUrl('/api/logs/stream').replace(/^http/, 'ws');
    wsRef.current = new WebSocket(wsUrl);

    wsRef.current.onopen = () => {
      setIsStreaming(true);
      console.log('WebSocket connected for log streaming');
    };

    wsRef.current.onmessage = (event) => {
      try {
        const logEntry = JSON.parse(event.data);
        setLogs(prev => [...prev, logEntry].slice(-500)); // 最新500件を保持
      } catch (error) {
        console.error('Failed to parse log entry:', error);
      }
    };

    wsRef.current.onclose = () => {
      setIsStreaming(false);
      console.log('WebSocket disconnected');
    };

    wsRef.current.onerror = (error) => {
      console.error('WebSocket error:', error);
      setIsStreaming(false);
    };
  };

  const stopStreaming = () => {
    if (wsRef.current) {
      wsRef.current.close();
      wsRef.current = null;
    }
    setIsStreaming(false);
  };

  const clearLogs = async () => {
    try {
      const response = await fetch(buildApiUrl('/api/logs/clear'), {
        method: 'POST',
      });
      if (response.ok) {
        setLogs([]);
      }
    } catch (error) {
      console.error('Failed to clear logs:', error);
    }
  };

  const downloadLogs = async (format: 'json' | 'text') => {
    const url = buildApiUrl(`/api/logs/download?format=${format}`);
    window.open(url, '_blank');
  };

  const filteredLogs = logs.filter(log => {
    if (levelFilter !== 'all' && log.level !== levelFilter) {
      return false;
    }
    if (filter && !log.message.toLowerCase().includes(filter.toLowerCase())) {
      return false;
    }
    return true;
  });

  const getLevelColor = (level: string) => {
    switch (level.toLowerCase()) {
      case 'error':
        return 'text-red-600';
      case 'warn':
        return 'text-yellow-600';
      case 'info':
        return 'text-blue-600';
      case 'debug':
        return 'text-gray-600';
      default:
        return 'text-gray-800';
    }
  };

  const formatTimestamp = (timestamp: string) => {
    const date = new Date(timestamp);
    return date.toLocaleTimeString('ja-JP', { 
      hour: '2-digit', 
      minute: '2-digit', 
      second: '2-digit',
      fractionalSecondDigits: 3 
    } as any);
  };

  const containerClass = embedded ? '' : 'container mx-auto p-4';

  return (
    <div className={containerClass}>
      <Card>
        <CardHeader>
          <div className="flex justify-between items-center">
            <div>
              <CardTitle>ログビューアー</CardTitle>
              <CardDescription>
                システムログの表示とダウンロード
              </CardDescription>
            </div>
            <div className="flex space-x-2">
              {isStreaming ? (
                <Button onClick={stopStreaming} variant="outline" size="sm">
                  <Pause className="h-4 w-4 mr-2" />
                  停止
                </Button>
              ) : (
                <Button onClick={startStreaming} variant="outline" size="sm">
                  <Play className="h-4 w-4 mr-2" />
                  リアルタイム
                </Button>
              )}
              <Button onClick={fetchLogs} variant="outline" size="sm">
                更新
              </Button>
            </div>
          </div>
        </CardHeader>
        <CardContent>
          {/* フィルター */}
          <div className="flex space-x-2 mb-4">
            <div className="flex-1">
              <Input
                placeholder="メッセージで検索..."
                value={filter}
                onChange={(e) => setFilter(e.target.value)}
                className="w-full"
              />
            </div>
            <Select value={levelFilter} onValueChange={setLevelFilter}>
              <SelectTrigger className="w-32">
                <SelectValue placeholder="レベル" />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="all">すべて</SelectItem>
                <SelectItem value="error">Error</SelectItem>
                <SelectItem value="warn">Warn</SelectItem>
                <SelectItem value="info">Info</SelectItem>
                <SelectItem value="debug">Debug</SelectItem>
              </SelectContent>
            </Select>
            <Button
              onClick={() => setAutoScroll(!autoScroll)}
              variant={autoScroll ? 'default' : 'outline'}
              size="sm"
            >
              自動スクロール
            </Button>
          </div>

          {/* ログ表示エリア */}
          <div className="bg-gray-900 text-gray-100 rounded-lg p-4 h-96 overflow-y-auto font-mono text-sm">
            {filteredLogs.length === 0 ? (
              <div className="text-center text-gray-500">ログがありません</div>
            ) : (
              filteredLogs.map((log, index) => (
                <div key={index} className="mb-1 hover:bg-gray-800 px-2 py-1 rounded">
                  <span className="text-gray-400">{formatTimestamp(log.timestamp)}</span>
                  <span className={`ml-2 font-semibold ${getLevelColor(log.level)}`}>
                    [{log.level.toUpperCase()}]
                  </span>
                  <span className="ml-2">{log.message}</span>
                  {log.fields && Object.keys(log.fields).length > 0 && (
                    <span className="ml-2 text-gray-500">
                      {JSON.stringify(log.fields)}
                    </span>
                  )}
                </div>
              ))
            )}
            <div ref={logsEndRef} />
          </div>

          {/* アクションボタン */}
          <div className="flex justify-between items-center mt-4">
            <div className="text-sm text-gray-600">
              {filteredLogs.length} / {logs.length} ログ
              {isStreaming && ' (ストリーミング中)'}
            </div>
            <div className="flex space-x-2">
              <Button onClick={clearLogs} variant="outline" size="sm">
                <Trash2 className="h-4 w-4 mr-2" />
                クリア
              </Button>
              <Button onClick={() => downloadLogs('text')} variant="outline" size="sm">
                <Download className="h-4 w-4 mr-2" />
                TXT
              </Button>
              <Button onClick={() => downloadLogs('json')} variant="outline" size="sm">
                <Download className="h-4 w-4 mr-2" />
                JSON
              </Button>
            </div>
          </div>
        </CardContent>
      </Card>
    </div>
  );
};