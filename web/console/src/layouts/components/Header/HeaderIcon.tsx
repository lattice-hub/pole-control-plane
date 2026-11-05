import React, { memo } from 'react';
import { useTranslation } from 'react-i18next';
import { useNavigate } from 'components/Router';
import { Button, Popup, Badge, Dropdown, Space } from 'components/Fluent';
import {
  AlphaIcon,
  Icon,
  HelpCircleIcon,
  SettingIcon,
  PoweroffIcon,
  UserCircleIcon,
} from 'components/Fluent/icons';
import { useAppDispatch, useAppSelector } from 'modules/store';
import { toggleSetting } from 'modules/global';
import { logout } from 'modules/user/login';
import Style from './HeaderIcon.module.less';

const { DropdownMenu, DropdownItem } = Dropdown;

export default memo(() => {
  const dispatch = useAppDispatch();
  const navigate = useNavigate();
  const userInfo = useAppSelector((state) => state.userLogin.currentUser);
  const { i18n, t } = useTranslation();

  const gotoWiki = () => {
    window.open('https://github.com/pole-io/pole-control-plane');
  };

  const clickHandler = (data: any) => {
    if (data.value === 1) {
      navigate('/namespace');
    }
  };
  const handleLogout = async () => {
    await dispatch(logout());
    navigate('/login');
  };

  return (
    <Space align='center'>
      <Dropdown
        trigger={'click'}
        placement='bottom'
        minColumnWidth={100}
        maxHeight={120}
        options={[
          { content: t('menu.language.zh'), value: 'zh-CN' },
          { content: t('menu.language.en'), value: 'en-US' },
        ]}
        onClick={({ value }) => {
          if (typeof value === 'string') i18n.changeLanguage(value);
        }}
      >
        <Button variant='text' className={Style.languageButton} aria-label='语言' icon={<AlphaIcon />}>
          <span className={Style.languageText}>{t('menu.language')}</span>
        </Button>
      </Dropdown>
      <Popup content='帮助文档' placement='bottom' showArrow destroyOnClose>
        <Button
          aria-label='打开帮助文档'
          className={Style.menuIcon}
          shape='square'
          size='large'
          variant='text'
          onClick={gotoWiki}
          icon={<HelpCircleIcon />}
        />
      </Popup>
      <Dropdown trigger={'click'} onClick={clickHandler}>
        <Button variant='text' className={Style.dropdown}>
          <Icon name='user-circle' className={`${Style.icon} ${Style.userIcon}`} />
          <span className={Style.text}>{userInfo.name}</span>
          <Icon name='chevron-down' className={`${Style.icon} ${Style.chevronIcon}`} />
        </Button>
        <DropdownMenu>
          <DropdownItem value={1}>
            <div className={Style.dropItem}>
              <UserCircleIcon />
              <span>个人中心</span>
            </div>
          </DropdownItem>
          <DropdownItem value={1} onClick={handleLogout}>
            <div className={Style.dropItem}>
              <PoweroffIcon />
              <span>退出登录</span>
            </div>
          </DropdownItem>
        </DropdownMenu>
      </Dropdown>
      <Popup content='页面设置' placement='bottom' showArrow destroyOnClose>
        <Button
          aria-label='打开页面设置'
          className={Style.menuIcon}
          shape='square'
          size='large'
          variant='text'
          onClick={() => dispatch(toggleSetting())}
          icon={<SettingIcon />}
        />
      </Popup>
    </Space>
  );
});
