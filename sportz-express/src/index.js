import AgentAPI from "apminsight";
AgentAPI.config();

import express from 'express';
import cors from 'cors';
import http from 'http';
import { matchRouter } from './routes/matches.js';
import { attachWebSocketServer } from './websocket/server.js';
import { securityMiddleware } from './arcjet.js';
import { commentaryRouter } from './routes/commentary.js';

const PORT = Number(process.env.PORT) || 8000;
const HOST = process.env.HOST || '0.0.0.0';

const app = express();
const server = http.createServer(app);

app.use(express.json());
// Enable CORS for the frontend dev server
const VITE_FRONTEND_URL = process.env.VITE_FRONTEND_URL || 'http://localhost:5173';
app.use(cors({ origin: VITE_FRONTEND_URL }));

app.get('/', (req, res) => {
    res.send('Welcome to Sportz Express API!');
});

// app.use(securityMiddleware());

app.use('/matches', matchRouter);
app.use('/matches/:id/commentary', commentaryRouter)

const { broadcastMatchCreated, broadcastCommentary, broadcastScoreUpdate } = attachWebSocketServer(server);
app.locals.broadcastMatchCreated = broadcastMatchCreated;
app.locals.broadcastCommentary = broadcastCommentary;
app.locals.broadcastScoreUpdate = broadcastScoreUpdate;

server.listen(PORT, HOST, () => {
    const baseUrl = HOST === '0.0.0.0' ? `http://localhost:${PORT}` : `http://${HOST}:${PORT}`;
    console.log(`Server is running on ${baseUrl}`);
    console.log(`Websocket server is running on ${baseUrl.replace('http', 'ws')}/websocket`);
})