/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { Link } from '@tanstack/react-router'
import { ArrowRight } from 'lucide-react'
import { useTranslation } from 'react-i18next'

interface HeroProps {
  className?: string
  isAuthenticated?: boolean
}

export function Hero(_props: HeroProps) {
  const { t } = useTranslation()

  return (
    <section className='relative w-full overflow-hidden bg-white'>
      {/* Container with 1920:1080 aspect ratio - scales proportionally */}
      <div
        className='relative w-full'
        style={{ aspectRatio: '1920 / 1080' }}
      >
        {/* Background image */}
        <img
          src='https://www.free2walk.cn/free2cloud/public/home-bg.png'
          alt=''
          aria-hidden
          className='absolute inset-0 h-full w-full object-cover'
        />

        {/* AI icon - 660x660 at (1152, 240) */}
        <img
          src='https://www.free2walk.cn/free2cloud/public/home-ai-icon.png'
          alt=''
          aria-hidden
          className='absolute object-contain'
          style={{
            left: '60%',
            top: '22.2%',
            width: '34.4%',
            height: '61.1%',
          }}
        />

        {/* Logo - 284x284 at (860, 544) */}
        <img
          src='https://www.free2walk.cn/free2cloud/public/home-logo.png'
          alt=''
          aria-hidden
          className='absolute object-contain'
          style={{
            left: '44.8%',
            top: '50.4%',
            width: '14.8%',
            height: '26.3%',
          }}
        />

        {/* Badge - 170x48 at (94, 256), rx=24 */}
        <div
          className='absolute flex items-center justify-center rounded-full border'
          style={{
            left: '4.9%',
            top: '23.7%',
            width: '8.85%',
            height: '4.44%',
            backgroundColor: 'rgba(215, 236, 255, 0.5)',
            borderColor: 'rgb(193, 220, 255)',
            borderWidth: '1px',
          }}
        >
          <span
            className='font-medium whitespace-nowrap'
            style={{
              color: 'rgb(22, 93, 252)',
              fontSize: 'clamp(9px, 1.2vw, 18px)',
            }}
          >
            {t('AI Application Infrastructure Foundation')}
          </span>
        </div>

        {/* Headline line 1 - black text, y=377-464 in SVG */}
        <h1
          className='absolute font-bold whitespace-nowrap'
          style={{
            left: '5%',
            top: '34.9%',
            margin: 0,
            color: 'rgb(7, 7, 7)',
            fontSize: 'clamp(22px, 3vw, 64px)',
            lineHeight: 1.0,
            letterSpacing: '0em',
          }}
        >
          {t('Unified API Gateway for')}
        </h1>

        {/* Headline line 2 - gradient text, y=485-560 in SVG */}
        <h2
          className='absolute font-bold whitespace-nowrap'
          style={{
            left: '5%',
            top: '42%',
            margin: 0,
            fontSize: 'clamp(22px, 3vw, 64px)',
            lineHeight: 1.0,
            letterSpacing: '0em',
            background:
              'linear-gradient(to right, rgb(87, 161, 255), rgb(163, 134, 255) 48%, rgb(173, 74, 255))',
            WebkitBackgroundClip: 'text',
            backgroundClip: 'text',
            WebkitTextFillColor: 'transparent',
          }}
        >
          {t('Vast Range of AI Models')}
        </h2>

        {/* Description - gray text, y=617-681 in SVG (2 lines) */}
        <p
          className='absolute'
          style={{
            left: '4.95%',
            top: '57.1%',
            margin: 0,
            maxWidth: '36.5%',
            color: 'rgb(51, 51, 51)',
            fontSize: 'clamp(11px, 1.1vw, 21px)',
            lineHeight: 1.5,
          }}
        >
          {t(
            'Access a vast selection of models via a standard, unified API protocol. Power AI applications, manage digital assets, and connect the Future.'
          )}
        </p>

        {/* Button - 222x72 at (94, 726), rx=20, black bg */}
        <div
          className='absolute'
          style={{
            left: '4.9%',
            top: '67.2%',
            width: '11.56%',
            minWidth: '120px',
            height: '6.67%',
            minHeight: '44px',
          }}
        >
          <Link
            to='/dashboard'
            className='flex h-full w-full items-center justify-center gap-1.5 rounded-[20px] text-white transition-opacity hover:opacity-85'
            style={{ backgroundColor: 'rgb(7, 7, 7)' }}
          >
            <span
              className='font-medium'
              style={{ fontSize: 'clamp(12px, 1.3vw, 20px)' }}
            >
              {t('Get Started')}
            </span>
            <ArrowRight
              style={{
                width: 'clamp(13px, 1.1vw, 18px)',
                height: 'clamp(13px, 1.1vw, 18px)',
              }}
            />
          </Link>
        </div>
      </div>
    </section>
  )
}
