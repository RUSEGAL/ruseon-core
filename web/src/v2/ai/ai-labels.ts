/**
 * COCO 80 Classes Mapping & Localization (RU / EN)
 * Complete classification with category-based styling.
 */

export interface CocoClassInfo {
  id: number;
  en: string;
  ru: string;
  category: 'person' | 'vehicle' | 'animal' | 'object';
  color: string;
}

export const COCO_CLASSES: string[] = [
  'person', 'bicycle', 'car', 'motorcycle', 'airplane', 'bus', 'train', 'truck', 'boat',
  'traffic light', 'fire hydrant', 'stop sign', 'parking meter', 'bench', 'bird', 'cat',
  'dog', 'horse', 'sheep', 'cow', 'elephant', 'bear', 'zebra', 'giraffe', 'backpack',
  'umbrella', 'handbag', 'tie', 'suitcase', 'frisbee', 'skis', 'snowboard', 'sports ball',
  'kite', 'baseball bat', 'baseball glove', 'skateboard', 'surfboard', 'tennis racket',
  'bottle', 'wine glass', 'cup', 'fork', 'knife', 'spoon', 'bowl', 'banana', 'apple',
  'sandwich', 'orange', 'broccoli', 'carrot', 'hot dog', 'pizza', 'donut', 'cake',
  'chair', 'couch', 'potted plant', 'bed', 'dining table', 'toilet', 'tv', 'laptop',
  'mouse', 'remote', 'keyboard', 'cell phone', 'microwave', 'oven', 'toaster', 'sink',
  'refrigerator', 'book', 'clock', 'vase', 'scissors', 'teddy bear', 'hair drier', 'toothbrush'
];

export const COCO_LABELS_RU: Record<string, string> = {
  person: 'Человек',
  bicycle: 'Велосипед',
  car: 'Автомобиль',
  motorcycle: 'Мотоцикл',
  airplane: 'Самолет',
  bus: 'Автобус',
  train: 'Поезд',
  truck: 'Грузовик',
  boat: 'Катер/Лодка',
  'traffic light': 'Светофор',
  'fire hydrant': 'Гидрант',
  'stop sign': 'Знак STOP',
  'parking meter': 'Паркомат',
  bench: 'Скамейка',
  bird: 'Птица',
  cat: 'Кошка',
  dog: 'Собака',
  horse: 'Лошадь',
  sheep: 'Овца',
  cow: 'Корова',
  elephant: 'Слон',
  bear: 'Медведь',
  zebra: 'Зебра',
  giraffe: 'Жираф',
  backpack: 'Рюкзак',
  umbrella: 'Зонт',
  handbag: 'Сумка',
  tie: 'Галстук',
  suitcase: 'Чемодан',
  frisbee: 'Фрисби',
  skis: 'Лыжи',
  snowboard: 'Сноуборд',
  'sports ball': 'Мяч',
  kite: 'Воздушный змей',
  'baseball bat': 'Бита',
  'baseball glove': 'Перчатка',
  skateboard: 'Скейтборд',
  surfboard: 'Серфборд',
  'tennis racket': 'Ракетка',
  bottle: 'Бутылка',
  'wine glass': 'Бокал',
  cup: 'Чашка',
  fork: 'Вилка',
  knife: 'Нож',
  spoon: 'Ложка',
  bowl: 'Миска',
  banana: 'Банан',
  apple: 'Яблоко',
  sandwich: 'Сэндвич',
  orange: 'Апельсин',
  broccoli: 'Брокколи',
  carrot: 'Морковь',
  'hot dog': 'Хот-дог',
  pizza: 'Пицца',
  donut: 'Пончик',
  cake: 'Торт',
  chair: 'Стул',
  couch: 'Диван',
  'potted plant': 'Растение',
  bed: 'Кровать',
  'dining table': 'Стол',
  toilet: 'Туалет',
  tv: 'Экран/ТВ',
  laptop: 'Ноутбук',
  mouse: 'Мышь',
  remote: 'Пульт',
  keyboard: 'Клавиатура',
  'cell phone': 'Телефон',
  microwave: 'Микроволновка',
  oven: 'Духовка',
  toaster: 'Тостер',
  sink: 'Раковина',
  refrigerator: 'Холодильник',
  book: 'Книга',
  clock: 'Часы',
  vase: 'Ваза',
  scissors: 'Ножницы',
  'teddy bear': 'Игрушка',
  'hair drier': 'Фен',
  toothbrush: 'Зубная щетка',
};

export function getAiClassColor(classId: number): string {
  if (classId === 0) return '#22c55e'; // Person -> Vibrant Green
  if (classId >= 1 && classId <= 8) return '#38bdf8'; // Vehicle -> Vibrant Cyan / Blue
  if (classId >= 14 && classId <= 23) return '#f97316'; // Animal -> Vibrant Orange
  return '#eab308'; // Other / Electronics / Objects -> Amber Yellow
}

export function getLocalizedClassLabel(label: string): string {
  return COCO_LABELS_RU[label] || label;
}
