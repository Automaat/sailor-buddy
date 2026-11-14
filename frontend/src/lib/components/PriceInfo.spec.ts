import { describe, it, expect } from 'vitest';
import { render, screen } from '@testing-library/svelte';
import PriceInfo from './PriceInfo.svelte';

const wrm = { currency: 'PLN' as const, cost_per_person: 820, cost_per_person_non_member: 870 };

describe('PriceInfo', () => {
	it.each([
		['czlonek', '820 PLN'],
		['sympatyk', '820 PLN'],
		['gosc', '870 PLN']
	] as const)('shows %s their price as Twoja cena', (membership, want) => {
		render(PriceInfo, { priced: wrm, membership });
		expect(screen.getByTestId('your-price')).toHaveTextContent(`Twoja cena: ${want}`);
		expect(screen.getByText('Członkowie i sympatycy')).toBeInTheDocument();
		expect(screen.getByText('Goście')).toBeInTheDocument();
	});

	it.each(['czlonek', 'sympatyk', 'gosc'] as const)(
		'shows %s the single price when only one is set',
		(membership) => {
			render(PriceInfo, {
				priced: { currency: 'EUR', cost_per_person: 760, yacht_fund: 150 },
				membership
			});
			expect(screen.getByTestId('your-price')).toHaveTextContent('Twoja cena: 760 EUR');
			expect(screen.queryByText('Goście')).not.toBeInTheDocument();
			expect(screen.getByText('Kasa jachtowa (szacunkowo)').nextElementSibling).toHaveTextContent(
				'150 EUR'
			);
		}
	);

	it('shows both tiers without Twoja cena when membership is unknown', () => {
		render(PriceInfo, { priced: wrm, membership: undefined });
		expect(screen.queryByTestId('your-price')).not.toBeInTheDocument();
		expect(screen.getByText('Goście').nextElementSibling).toHaveTextContent('870 PLN');
	});

	it('lists what is and is not included', () => {
		render(PriceInfo, {
			priced: { currency: 'EUR' },
			membership: 'czlonek',
			included: ['jacht', 'skipper'],
			notIncluded: ['dojazd']
		});
		expect(screen.getByText('W cenie')).toBeInTheDocument();
		expect(screen.getByTestId('price-included').querySelectorAll('li')).toHaveLength(2);
		expect(screen.getByText('Poza ceną')).toBeInTheDocument();
		expect(screen.getByTestId('price-not-included')).toHaveTextContent('dojazd');
		expect(screen.queryByTestId('your-price')).not.toBeInTheDocument();
	});

	it('renders nothing for an unpriced cruise', () => {
		const { container } = render(PriceInfo, {
			priced: { currency: 'PLN', cost_per_person: 0 },
			membership: 'gosc',
			included: [],
			notIncluded: null
		});
		expect(container.querySelector('section')).toBeNull();
	});
});

describe('PriceInfo — free member berth', () => {
	it('shows a member 0 and lists both tiers when guests pay', () => {
		render(PriceInfo, {
			priced: { currency: 'PLN', cost_per_person: 0, cost_per_person_non_member: 500 },
			membership: 'czlonek'
		});
		expect(screen.getByTestId('your-price')).toHaveTextContent('Twoja cena: 0 PLN');
		expect(screen.getByText('Goście').nextElementSibling).toHaveTextContent('500 PLN');
	});
});
