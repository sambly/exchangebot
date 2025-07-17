import { timeToLocal } from './help.js';

import { createChart } from 'lightweight-charts';


export function lw_charts_volume(container_chart, chartOptions, pair, chartData) {

    container_chart.innerHTML = '';
    container_chart.style.position = 'relative';

    const chartColors = ['#2962FF', '#FF5733', '#8c7401', '#5733FF', '#FF33E9','#23605f']; // Массив цветов для графиков
    const legendItems = []; // Массив для хранения меток графиков


    const chart = createChart(container_chart, chartOptions);
    chart.applyOptions({
        rightPriceScale: {
            scaleMargins: {
                top: 0.1,
                bottom: 0.1,
            },
        },
    });

    let colorIndex = 0;
    let lineSeriesList = [];

    for (let key in chartData) {
        if (chartData.hasOwnProperty(key)) {
            const lineSeries = chart.addLineSeries({ color: chartColors[colorIndex] });   
            lineSeries.setData(chartData[key].value);
            
            lineSeriesList.push(lineSeries);
            // Добавляем метку графика в легенду
            legendItems.push({label: key,color: chartColors[colorIndex]});
            colorIndex = colorIndex + 1;
        }
    }
    
    // Отображение легенды
    const toolTip = document.createElement('div');
    toolTip.className = 'three-line-legend';
    container_chart.appendChild(toolTip);
    toolTip.style.display = 'block';
    toolTip.style.left = 3 + 'px';
    toolTip.style.top = 3 + 'px';

    // Создание HTML-кода для легенды на основе данных о метках графиков
    let legendHTML = '';
    legendHTML = '<div style="margin: 4px 0px; color: #000000; font-weight: bold;">' + pair + '</div>';
    legendItems.forEach(item => {
        legendHTML += `<div style="margin: 4px 0px; color: ${item.color}">${item.label}</div>`;
    });

    toolTip.innerHTML = legendHTML;
   
    let priceFormatted = [];
    let time = '';
    chart.subscribeCrosshairMove(param => {
        for (let i = 0; i < lineSeriesList.length; i++) {
            priceFormatted.push('');
        }
        if (param.time) {
            time = param.time;
            for (let i = 0; i < lineSeriesList.length; i++) {
                const data = param.seriesData.get(lineSeriesList[i]);
                const price = data.value !== undefined ? data.value : data.close;
                priceFormatted[i] = price.toLocaleString('en-US', { useGrouping: true, maximumFractionDigits: 0 }).replace(/,/g, ' ');
            }
        }
        legendHTML = '<div style="margin: 4px 0px; color: #000000; font-weight: bold;">' + pair + '</div>';
        for (let i = 0; i < legendItems.length; i++) {   
            legendHTML += `<div style="margin: 4px 0px; color: ${legendItems[i].color}">${legendItems[i].label}  ${priceFormatted[i]}</div>`;
        }
        const options = { year: 'numeric', month: '2-digit', day: '2-digit',hour: '2-digit',minute: '2-digit',second: '2-digit',hour12: false};
        // const forпmattedDate = new Date(time * 1000).toLocaleString('en-US', options); 
        // legendHTML += '<div style="margin: 4px 0px; color: #000000; font-weight: bold;">' + forпmattedDate + '</div>';

        toolTip.innerHTML = legendHTML;
    });

    chart.timeScale().fitContent();

}


export function lw_charts_orders(container_chart, chartOptions, pair, orders, update_cadles) {
    return new Promise((resolve, reject) => {
        container_chart.innerHTML = '';
        container_chart.style.position = 'relative';

        let intervals = ['1m', '3m', '15m', '1h', '4h', '1d'];
        const switcherElement = createSimpleSwitcher(intervals, intervals[0], syncToInterval);

        const chart = createChart(container_chart, chartOptions);
        container_chart.appendChild(switcherElement);

        // Отображение легенды
        const toolTip = document.createElement('div');
        toolTip.className = 'three-line-legend';
        container_chart.appendChild(toolTip);
        toolTip.style.display = 'block';
        toolTip.style.left = 3 + 'px';
        toolTip.style.top = 3 + 'px';
        toolTip.innerHTML = '<div style="margin: 4px 0px; color: #000000; font-weight: bold;">' + pair.value + '</div>';

        var candleSeries = null;

        function syncToInterval(interval) {
            if (candleSeries) {
                chart.removeSeries(candleSeries);
                candleSeries = null;
            }

            candleSeries = chart.addCandlestickSeries();

            update_cadles(pair.value, interval).then(candles => {
                candleSeries.setData(candles);

                // Отображение ордеров на графике
                let markers_chart = [];

                for (let order of orders) {

                    if (order.Pair === pair.value) {

                        let timeCreated = timeToLocal(new Date(order.TimeCreated) / 1000);
                        let timeFinished = timeToLocal(new Date(order.Time) / 1000);

                        if (order.Side == 'BUY') {
                            if (order.Status != 'Close') {
                                markers_chart.push({ time: timeCreated, position: 'belowBar', color: '#00ff00', shape: 'arrowUp', text: `long ${order.ID}` });
                            } else {
                                markers_chart.push({ time: timeCreated, position: 'belowBar', color: '#00ff00', shape: 'arrowUp', text: `long ${order.ID}` });
                                markers_chart.push({ time: timeFinished, position: 'aboveBar', color: '#e91e63', shape: 'arrowDown', text: `long ${order.ID}` });
                            }
                        }
                        if (order.Side == 'SELL') {
                            if (order.Status != 'Close') {
                                markers_chart.push({ time: timeCreated, position: 'belowBar', color: '#00ff00', shape: 'arrowDown', text: `short ${order.ID}` });
                            } else {
                                markers_chart.push({ time: timeCreated, position: 'belowBar', color: '#00ff00', shape: 'arrowDown', text: `short ${order.ID}` });
                                markers_chart.push({ time: timeFinished, position: 'aboveBar', color: '#e91e63', shape: 'arrowUp', text: `short ${order.ID}` });
                            }
                        }
                    }
                }

                candleSeries.setMarkers(markers_chart);
                resolve();
            }).catch(error => {
                reject(error);
            });
        }

        syncToInterval(intervals[2]);

        let price = '';
        chart.subscribeCrosshairMove(param => {

            if (param.time) {
                const data = param.seriesData.get(candleSeries);
                price = data.value !== undefined ? data.value : data.close;
            }
            toolTip.innerHTML = '<div style="margin: 4px 0px; color: #000000;font-weight: bold;">' + pair.value + '</div>';
            toolTip.innerHTML += `<div style="margin: 4px 0px; color: #2962FF">Value ${price}</div>`; 
       
        });

        chart.timeScale().fitContent();

    });
}



export function widget_charts(container_chart, pair) {

    localStorage.setItem('widgetPair', pair);
    let chartWidth = container_chart.clientWidth;
    const params = new URLSearchParams(window.location.search);
    const paramsPeriod = params.get("period");
    const intervalMap = {
        '1m': '1',
        '3m': '3',
        '15m': '15',
        '1h': '60',
        '4h': '240',
        '1d': '1D'
    };

    const intervals = Object.keys(intervalMap); // ['1m', '3m', '15m', '1h', '4h', '1d']
    let selectedInterval = '15'; // Значение по умолчанию (15 минут)

    if (paramsPeriod && intervals.includes(paramsPeriod)) {
        selectedInterval = intervalMap[paramsPeriod];
    }

    new TradingView.widget(
        {
            "height": "532",
            width: chartWidth,
            "symbol": "BINANCE:" + pair,
            "interval": selectedInterval,
            "timezone": "Europe/Moscow",
            "theme": "Light",
            "style": "1",
            "locale": "ru",
            "toolbar_bg": "#f1f3f6",
            "enable_publishing": false,
            "allow_symbol_change": true,
            "container_id": "tradingview_3418f"
        }
    );
}



function createSimpleSwitcher(items, activeItem, activeItemChangedCallback) {
    var switcherElement = document.createElement('div');
    switcherElement.classList.add('switcher');

    var intervalElements = items.map(function (item) {
        var itemEl = document.createElement('button');
        itemEl.innerText = item;
        itemEl.classList.add('switcher-item');
        itemEl.classList.toggle('switcher-active-item', item === activeItem);
        itemEl.addEventListener('click', function () {
            onItemClicked(item);
        });
        switcherElement.appendChild(itemEl);
        return itemEl;
    });

    function onItemClicked(item) {
        if (item === activeItem) {
            return;
        }

        intervalElements.forEach(function (element, index) {
            element.classList.toggle('switcher-active-item', items[index] === item);
        });

        activeItem = item;

        activeItemChangedCallback(item);
    }

    return switcherElement;
}