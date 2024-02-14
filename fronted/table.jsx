import React from 'react';
import ReactDOM from 'react-dom/client';
import { PrimeReactProvider } from 'primereact/api';
import 'primeflex/primeflex.css';  
import 'primereact/resources/primereact.css';
import 'primereact/resources/themes/lara-light-indigo/theme.css';

import App from './App';


export function Tickers_list(){



    // Изменение высоты блоков
    let ch_price = document.querySelector('#list').clientHeight - document.querySelector('#list-top').clientHeight;
    document.querySelector('#list-ch-price').style.height = `${ch_price}px`;

    const root = ReactDOM.createRoot(document.getElementById('list-ch-price'));

    root.render(
    <React.StrictMode>
        <PrimeReactProvider>
        <App />
        </PrimeReactProvider>
    </React.StrictMode>
    );


}


